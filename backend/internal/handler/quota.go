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
	Date      string `json:"date"`      // YYYY-MM-DD (UTC)
	Used      int    `json:"used"`      // interpretations used today
	Remaining int    `json:"remaining"` // remaining quota (may be -1 for unlimited)
	Limit     int    `json:"limit"`     // daily limit (may be -1 for unlimited)
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
	if limit < 0 {
		c.JSON(http.StatusOK, QuotaResponse{Date: today, Used: used, Remaining: -1, Limit: -1})
		return
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	c.JSON(http.StatusOK, QuotaResponse{
		Date:      today,
		Used:      used,
		Remaining: remaining,
		Limit:     limit,
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
	if idempotencyKey == "" {
		return IncrementUsage(db, userID, limit)
	}
	today := time.Now().UTC().Format("2006-01-02")
	return db.Transaction(func(tx *gorm.DB) error {
		reservation := model.AIRequestReservation{
			UserID:         userID,
			Date:           today,
			IdempotencyKey: idempotencyKey,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrDuplicateAIRequest
		}
		return IncrementUsage(tx, userID, limit)
	})
}

// ErrQuotaExceeded is returned when the user has used up their daily quota.
var ErrQuotaExceeded = errors.New("daily quota exceeded")

// ErrDuplicateAIRequest means the same user already submitted this key today.
var ErrDuplicateAIRequest = errors.New("duplicate AI request")
