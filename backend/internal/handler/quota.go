package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"predictdestiny/internal/auth"
	"predictdestiny/internal/model"
)

// QuotaHandler handles quota query endpoints.
type QuotaHandler struct {
	DB *gorm.DB
}

// QuotaResponse is returned by GET /api/quota.
type QuotaResponse struct {
	Date                string `json:"date"`      // YYYY-MM-DD (UTC)
	Used                int    `json:"used"`      // interpretations used today
	Remaining           int    `json:"remaining"` // remaining quota (may be -1 for unlimited)
	Limit               int    `json:"limit"`     // daily limit (may be -1 for unlimited)
	EstimatedCostMicros int64  `json:"estimatedCostMicros"`
	ReservedCostMicros  int64  `json:"reservedCostMicros"`
	CostBudgetMicros    int64  `json:"costBudgetMicros"`
	CostRemainingMicros int64  `json:"costRemainingMicros"`
}

// Get returns the authenticated user's quota status for today.
// TODO: Once MembershipTier is implemented, this should look up the user's tier
// to determine the actual limit. For now, it reads from the settings table.
func (h *QuotaHandler) Get(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	today := time.Now().UTC().Format("2006-01-02")

	// Look up today's usage and the user's effective membership.
	var quota model.UsageQuota
	err := h.DB.Where("user_id = ? AND date = ?", userID, today).First(&quota).Error

	used := 0
	if err == nil {
		used = quota.Count
	}

	ent, entitlementErr := effectiveEntitlement(h.DB, userID, time.Now())
	if entitlementErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "membership unavailable"})
		return
	}
	limit := ent.Tier.DailyQuota
	var dailyCost model.AIDailyCostUsage
	_ = h.DB.Where("user_id = ? AND date = ?", userID, today).First(&dailyCost).Error
	estimatedCost := dailyCost.SpentMicros
	committedCost := dailyCost.SpentMicros + dailyCost.ReservedMicros
	costRemaining := int64(-1)
	if ent.Tier.DailyCostBudgetMicros >= 0 {
		costRemaining = ent.Tier.DailyCostBudgetMicros - committedCost
		if costRemaining < 0 {
			costRemaining = 0
		}
	}
	if limit < 0 {
		c.JSON(http.StatusOK, QuotaResponse{Date: today, Used: used, Remaining: -1, Limit: -1,
			EstimatedCostMicros: estimatedCost, ReservedCostMicros: dailyCost.ReservedMicros,
			CostBudgetMicros: ent.Tier.DailyCostBudgetMicros, CostRemainingMicros: costRemaining})
		return
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	c.JSON(http.StatusOK, QuotaResponse{
		Date:                today,
		Used:                used,
		Remaining:           remaining,
		Limit:               limit,
		EstimatedCostMicros: estimatedCost,
		ReservedCostMicros:  dailyCost.ReservedMicros,
		CostBudgetMicros:    ent.Tier.DailyCostBudgetMicros,
		CostRemainingMicros: costRemaining,
	})
}

// IncrementUsage increments the usage counter for the authenticated user.
// Returns error if quota exceeded.
func IncrementUsage(db *gorm.DB, userID uint, limit int) error {
	if limit < 0 { // unlimited
		return nil
	}
	if limit == 0 {
		return ErrQuotaExceeded
	}
	today := time.Now().UTC().Format("2006-01-02")

	// One statement performs create/increment and enforces the ceiling. The
	// unique (user_id,date) index makes concurrent requests serialize here.
	table := db.NamingStrategy.TableName("UsageQuota")
	var count int
	query := fmt.Sprintf(`
		INSERT INTO %s (user_id, date, count, created_at, updated_at)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, date) DO UPDATE
		SET count = %s.count + 1, updated_at = CURRENT_TIMESTAMP
		WHERE %s.count < ?
		RETURNING count`, table, table, table)
	result := db.Raw(query, userID, today, limit).Scan(&count)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 || count == 0 {
		return ErrQuotaExceeded
	}
	return nil
}

// ReserveAIRequest atomically claims an optional idempotency key and reserves
// quota. Reusing a key is rejected instead of allowing another uncharged AI
// call. If quota reservation fails, the key insert rolls back as well.
func ReserveAIRequest(db *gorm.DB, userID uint, limit int, idempotencyKey string) error {
	return ReserveAIRequestWithCost(db, userID, limit, idempotencyKey, "", -1, 0)
}

// ReserveAIRequestWithCost atomically claims idempotency, request-count quota,
// and (when enabled) the conservative cost hold.
func ReserveAIRequestWithCost(db *gorm.DB, userID uint, limit int, idempotencyKey, requestID string, costBudget, costReserve int64) error {
	if costBudget >= 0 && (requestID == "" || costReserve <= 0) {
		return ErrCostPolicyUnavailable
	}
	if idempotencyKey == "" && costBudget < 0 {
		return IncrementUsage(db, userID, limit)
	}
	today := time.Now().UTC().Format("2006-01-02")
	return db.Transaction(func(tx *gorm.DB) error {
		if idempotencyKey != "" {
			reservation := model.AIRequestReservation{
				UserID: userID, Date: today, IdempotencyKey: idempotencyKey,
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrDuplicateAIRequest
			}
		}
		if err := IncrementUsage(tx, userID, limit); err != nil {
			return err
		}
		if costBudget >= 0 {
			if err := incrementCostReservation(tx, userID, today, costBudget, costReserve); err != nil {
				return err
			}
			reservation := model.AICostReservation{
				UserID: userID, Date: today, RequestID: requestID,
				ReservedMicros: costReserve, Status: "reserved",
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrDuplicateAIRequest
			}
		}
		return nil
	})
}

func incrementCostReservation(db *gorm.DB, userID uint, date string, budget, reserve int64) error {
	if reserve > budget {
		return ErrCostBudgetExceeded
	}
	table := db.NamingStrategy.TableName("AIDailyCostUsage")
	var total int64
	query := fmt.Sprintf(`
		INSERT INTO %s (user_id, date, reserved_micros, spent_micros, created_at, updated_at)
		VALUES (?, ?, ?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, date) DO UPDATE
		SET reserved_micros = %s.reserved_micros + ?, updated_at = CURRENT_TIMESTAMP
		WHERE %s.reserved_micros + %s.spent_micros + ? <= ?
		RETURNING reserved_micros + spent_micros`, table, table, table, table)
	result := db.Raw(query, userID, date, reserve, reserve, reserve, budget).Scan(&total)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 || total == 0 {
		return ErrCostBudgetExceeded
	}
	return nil
}

// ErrQuotaExceeded is returned when the user has used up their daily quota.
var ErrQuotaExceeded = errors.New("daily quota exceeded")

// ErrDuplicateAIRequest means the same user already submitted this key today.
var ErrDuplicateAIRequest = errors.New("duplicate AI request")

var ErrCostBudgetExceeded = errors.New("daily AI cost budget exceeded")

var ErrCostPolicyUnavailable = errors.New("AI cost policy unavailable")
