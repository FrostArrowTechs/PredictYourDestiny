package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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

	// Look up today's usage
	var quota model.UsageQuota
	err := h.DB.Where("user_id = ? AND date = ?", userID, today).First(&quota).Error

	used := 0
	if err == nil {
		used = quota.Count
	}

	// For now, use a default limit of 5.
	// TODO: Read from settings or user's membership tier.
	limit := 5
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
	today := time.Now().UTC().Format("2006-01-02")

	// Get or create quota record for today
	var quota model.UsageQuota
	err := db.Where("user_id = ? AND date = ?", userID, today).First(&quota).Error

	if err == gorm.ErrRecordNotFound {
		// Create new quota record
		quota = model.UsageQuota{
			UserID: userID,
			Date:   today,
			Count:  0,
		}
		if err := db.Create(&quota).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Check limit (unless unlimited)
	if limit > 0 && quota.Count >= limit {
		return ErrQuotaExceeded
	}

	// Increment
	return db.Model(&quota).Update("count", quota.Count+1).Error
}

// ErrQuotaExceeded is returned when the user has used up their daily quota.
var ErrQuotaExceeded = &QuotaExceededError{}

type QuotaExceededError struct{}

func (e *QuotaExceededError) Error() string {
	return "daily quota exceeded"
}