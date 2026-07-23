package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/auth"
	"predictdestiny/internal/fortune"
	"predictdestiny/internal/model"
)

// entitlement is the effective, server-authoritative access granted to a user.
// Missing and expired memberships deliberately fall back to the free tier.
type entitlement struct {
	Tier           model.MembershipTier
	ExpiresAt      *time.Time
	FellBackToFree bool
}

func effectiveEntitlement(db *gorm.DB, userID uint, now time.Time) (entitlement, error) {
	var membership model.UserMembership
	err := db.Preload("Tier").Where("user_id = ?", userID).First(&membership).Error
	if err == nil && membership.Tier != nil && membership.Tier.IsEnabled &&
		(membership.ExpiresAt == nil || membership.ExpiresAt.After(now)) {
		return entitlement{Tier: *membership.Tier, ExpiresAt: membership.ExpiresAt}, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return entitlement{}, err
	}

	var free model.MembershipTier
	if err := db.Where("code = ? AND is_enabled = ?", model.TierCodeFree, true).First(&free).Error; err != nil {
		return entitlement{}, err
	}
	return entitlement{Tier: free, FellBackToFree: true}, nil
}

// EntitlementHandler exposes the same effective access policy used by AI
// authorization so clients never have to infer membership behavior.
type EntitlementHandler struct {
	DB      *gorm.DB
	Gateway ai.Gateway
}

type EntitlementResponse struct {
	EffectiveTier         string          `json:"effectiveTier"`
	TierName              string          `json:"tierName"`
	ExpiresAt             *time.Time      `json:"expiresAt"`
	DailyQuota            int             `json:"dailyQuota"`
	DailyCostBudgetMicros int64           `json:"dailyCostBudgetMicros"`
	Features              []string        `json:"features"`
	AvailableModels       []ai.ModelEntry `json:"availableModels"`
	FellBackToFree        bool            `json:"fellBackToFree"`
}

func (h *EntitlementHandler) Get(c *gin.Context) {
	userID := auth.GetUserID(c)
	ent, err := effectiveEntitlement(h.DB, userID, time.Now())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "membership unavailable"})
		return
	}

	features := make([]string, 0)
	if ent.Tier.Features != "" {
		if err := json.Unmarshal([]byte(ent.Tier.Features), &features); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "membership features are invalid"})
			return
		}
	}

	models := make([]ai.ModelEntry, 0)
	if h.Gateway != nil {
		catalog := h.Gateway.ListModels()
		models = append(models, catalog.Free...)
		if canUsePaidModel(ent.Tier.Code) {
			models = append(models, catalog.Paid...)
		}
	}

	c.JSON(http.StatusOK, EntitlementResponse{
		EffectiveTier:         ent.Tier.Code,
		TierName:              ent.Tier.Name,
		ExpiresAt:             ent.ExpiresAt,
		DailyQuota:            ent.Tier.DailyQuota,
		DailyCostBudgetMicros: ent.Tier.DailyCostBudgetMicros,
		Features:              features,
		AvailableModels:       models,
		FellBackToFree:        ent.FellBackToFree,
	})
}

func canUsePaidModel(tierCode string) bool {
	return tierCode == model.TierCodeBasic || tierCode == model.TierCodePremium
}

// authorizeAIRequest selects a catalog model using the effective membership,
// then atomically reserves one daily interpretation. Attempts count once they
// reach the AI gateway; this avoids ambiguous refund races for streaming calls.
func authorizeAIRequest(c *gin.Context, db *gorm.DB, gateway ai.Gateway, requested, promptTier string) (string, bool) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return "", false
	}

	ent, err := effectiveEntitlement(db, userID, time.Now())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "membership unavailable"})
		return "", false
	}

	cat := gateway.ListModels()
	paidAllowed := canUsePaidModel(ent.Tier.Code)
	selected := ""
	if requested != "" {
		entry, found := cat.Find(requested)
		if !found {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown AI model"})
			return "", false
		}
		if entry.Tier == ai.TierPaid && !paidAllowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "model requires a paid membership"})
			return "", false
		}
		selected = entry.ID
	} else if promptTier == fortune.TierPaid && paidAllowed && len(cat.Paid) > 0 {
		selected = cat.Paid[0].ID
	} else if len(cat.Free) > 0 {
		selected = cat.Free[0].ID
	} else if paidAllowed && len(cat.Paid) > 0 {
		selected = cat.Paid[0].ID
	}
	if selected == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no authorized AI model configured"})
		return "", false
	}

	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(idempotencyKey) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "idempotency key is too long"})
		return "", false
	}
	requestIDValue, _ := c.Get("requestID")
	requestID := strings.TrimSpace(fmt.Sprint(requestIDValue))
	costReserve, err := requestCostReserve(db, selected, ent.Tier.DailyCostBudgetMicros, time.Now())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI cost policy is not configured for this model"})
		return "", false
	}
	if err := ReserveAIRequestWithCost(db, userID, ent.Tier.DailyQuota, idempotencyKey, requestID, ent.Tier.DailyCostBudgetMicros, costReserve); err != nil {
		if errors.Is(err, ErrQuotaExceeded) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "daily quota exceeded"})
		} else if errors.Is(err, ErrCostBudgetExceeded) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "daily AI cost budget exceeded"})
		} else if errors.Is(err, ErrDuplicateAIRequest) {
			c.JSON(http.StatusConflict, gin.H{"error": "duplicate AI request"})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quota unavailable"})
		}
		return "", false
	}
	if idempotencyKey != "" {
		c.Header("Idempotency-Key", idempotencyKey)
	}
	feature := strings.TrimSuffix(strings.TrimPrefix(c.FullPath(), "/api/"), "/interpret")
	metadata := ai.UsageMetadata{UserID: userID, RequestID: requestID, IdempotencyKey: idempotencyKey, Feature: feature}
	c.Request = c.Request.WithContext(ai.WithUsageMetadata(c.Request.Context(), metadata))
	return selected, true
}

func requestCostReserve(db *gorm.DB, modelID string, budget int64, now time.Time) (int64, error) {
	if budget < 0 {
		return 0, nil
	}
	var provider model.AIProvider
	if err := db.Where("is_default = ? AND is_enabled = ?", true, true).First(&provider).Error; err != nil {
		return 0, err
	}
	var price model.AIModelPriceVersion
	if err := db.Where("provider_id = ? AND model = ? AND effective_from <= ?", provider.ID, modelID, now).
		Order("effective_from DESC, id DESC").First(&price).Error; err != nil {
		return 0, err
	}
	if price.RequestReserveMicros <= 0 {
		return 0, ErrCostPolicyUnavailable
	}
	return price.RequestReserveMicros, nil
}
