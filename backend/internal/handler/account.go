package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/auth"
	"predictdestiny/internal/model"
)

type AccountHandler struct {
	DB *gorm.DB
}

type accountExport struct {
	FormatVersion      string                    `json:"formatVersion"`
	ExportedAt         time.Time                 `json:"exportedAt"`
	User               UserPayload               `json:"user"`
	Memberships        []model.UserMembership    `json:"memberships"`
	Records            []model.FortuneRecord     `json:"records"`
	Chats              []model.ChatHistory       `json:"chats"`
	Usage              []model.UsageQuota        `json:"usage"`
	NameAnalyses       []model.NameAnalysis      `json:"nameAnalyses"`
	AIUsage            []model.AIUsageLedger     `json:"aiUsage"`
	AICostUsage        []model.AIDailyCostUsage  `json:"aiCostUsage"`
	AICostReservations []model.AICostReservation `json:"aiCostReservations"`
}

func (h *AccountHandler) Export(c *gin.Context) {
	userID := auth.GetUserID(c)
	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	export := accountExport{FormatVersion: "predictdestiny-account-export-v1", ExportedAt: time.Now().UTC(), User: userToPayload(user)}
	queries := []struct {
		value any
	}{
		{&export.Memberships}, {&export.Records}, {&export.Chats}, {&export.Usage}, {&export.NameAnalyses},
		{&export.AIUsage}, {&export.AICostUsage}, {&export.AICostReservations},
	}
	for _, query := range queries {
		if err := h.DB.Where("user_id = ?", userID).Find(query.value).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export account data"})
			return
		}
	}
	c.Header("Content-Disposition", `attachment; filename="predictdestiny-account-data.json"`)
	c.JSON(http.StatusOK, export)
}

func (h *AccountHandler) ClearHistory(c *gin.Context) {
	userID := auth.GetUserID(c)
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.ChatHistory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.NameAnalysis{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&model.FortuneRecord{}).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear history"})
		return
	}
	c.Status(http.StatusNoContent)
	c.Writer.WriteHeaderNow()
}

type deleteAccountRequest struct {
	Password string `json:"password" binding:"required"`
}

func (h *AccountHandler) Delete(c *gin.Context) {
	userID := auth.GetUserID(c)
	var request deleteAccountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}
	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err := auth.CheckPassword(user.Password, request.Password); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "password is incorrect"})
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		for _, target := range []any{
			&model.ChatHistory{}, &model.FortuneRecord{}, &model.NameAnalysis{},
			&model.UsageQuota{}, &model.AIRequestReservation{}, &model.UserMembership{},
			&model.AIUsageLedger{},
			&model.AIDailyCostUsage{}, &model.AICostReservation{},
		} {
			if err := tx.Where("user_id = ?", userID).Delete(target).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&model.User{}, userID).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete account"})
		return
	}
	c.Status(http.StatusNoContent)
	c.Writer.WriteHeaderNow()
}

// PurgeExpiredUserData applies the declared retention policy. Account and
// membership rows are never removed by retention; only user-generated history
// and expired request-idempotency records are affected.
func PurgeExpiredUserData(db *gorm.DB, now time.Time, historyDays, reservationDays int) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if historyDays > 0 {
			cutoff := now.UTC().AddDate(0, 0, -historyDays)
			oldRecordIDs := tx.Model(&model.FortuneRecord{}).Select("id").Where("created_at < ?", cutoff)
			if err := tx.Where("created_at < ? OR record_id IN (?)", cutoff, oldRecordIDs).Delete(&model.ChatHistory{}).Error; err != nil {
				return err
			}
			if err := tx.Where("created_at < ?", cutoff).Delete(&model.FortuneRecord{}).Error; err != nil {
				return err
			}
			if err := tx.Where("created_at < ?", cutoff.Unix()).Delete(&model.NameAnalysis{}).Error; err != nil {
				return err
			}
		}
		if reservationDays > 0 {
			cutoff := now.UTC().AddDate(0, 0, -reservationDays)
			if err := tx.Where("created_at < ?", cutoff).Delete(&model.AIRequestReservation{}).Error; err != nil {
				return err
			}
			if err := tx.Where("created_at < ? AND status = ?", cutoff, "settled").Delete(&model.AICostReservation{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
