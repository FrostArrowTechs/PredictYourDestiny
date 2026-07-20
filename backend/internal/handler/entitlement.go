package handler

import (
	"encoding/json"
	"errors"
	"net/http"
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
	if err == nil && membership.Tier != nil &&
		(membership.ExpiresAt == nil || membership.ExpiresAt.After(now)) {
		return entitlement{Tier: *membership.Tier, ExpiresAt: membership.ExpiresAt}, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return entitlement{}, err
	}

	var free model.MembershipTier
	if err := db.Where("code = ?", model.TierCodeFree).First(&free).Error; err != nil {
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
	EffectiveTier   string          `json:"effectiveTier"`
	TierName        string          `json:"tierName"`
	ExpiresAt       *time.Time      `json:"expiresAt"`
	DailyQuota      int             `json:"dailyQuota"`
	Features        []string        `json:"features"`
	AvailableModels []ai.ModelEntry `json:"availableModels"`
	FellBackToFree  bool            `json:"fellBackToFree"`
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
		EffectiveTier:   ent.Tier.Code,
		TierName:        ent.Tier.Name,
		ExpiresAt:       ent.ExpiresAt,
		DailyQuota:      ent.Tier.DailyQuota,
		Features:        features,
		AvailableModels: models,
		FellBackToFree:  ent.FellBackToFree,
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

	if err := IncrementUsage(db, userID, ent.Tier.DailyQuota); err != nil {
		if errors.Is(err, ErrQuotaExceeded) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "daily quota exceeded"})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quota unavailable"})
		}
		return "", false
	}
	return selected, true
}
