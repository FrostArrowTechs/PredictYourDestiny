package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

// AdminTierHandler handles membership tier management endpoints.
type AdminTierHandler struct {
	DB *gorm.DB
}

// TierPayload is the API response for a tier.
type TierPayload struct {
	ID         uint   `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	DailyQuota int    `json:"dailyQuota"`
	Features   string `json:"features"`
	PriceMonth int    `json:"priceMonth"`
	SortOrder  int    `json:"sortOrder"`
}

// ListTiers returns all membership tiers.
func (h *AdminTierHandler) ListTiers(c *gin.Context) {
	var tiers []model.MembershipTier
	if err := h.DB.Order("sort_order, id").Find(&tiers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tiers"})
		return
	}

	payload := make([]TierPayload, len(tiers))
	for i, t := range tiers {
		payload[i] = TierPayload{
			ID:         t.ID,
			Code:       t.Code,
			Name:       t.Name,
			DailyQuota: t.DailyQuota,
			Features:   t.Features,
			PriceMonth: t.PriceMonth,
			SortOrder:  t.SortOrder,
		}
	}

	c.JSON(http.StatusOK, gin.H{"tiers": payload})
}

// CreateTierRequest is the payload for POST /api/admin/tiers.
type CreateTierRequest struct {
	Code       string `json:"code" binding:"required"`
	Name       string `json:"name" binding:"required"`
	DailyQuota int    `json:"dailyQuota"`
	Features   string `json:"features"`
	PriceMonth int    `json:"priceMonth"`
	SortOrder  int    `json:"sortOrder"`
}

// CreateTier creates a new membership tier.
func (h *AdminTierHandler) CreateTier(c *gin.Context) {
	var req CreateTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if code already exists
	var existing model.MembershipTier
	if err := h.DB.Where("code = ?", req.Code).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "tier code already exists"})
		return
	}

	tier := model.MembershipTier{
		Code:       req.Code,
		Name:       req.Name,
		DailyQuota: req.DailyQuota,
		Features:   req.Features,
		PriceMonth: req.PriceMonth,
		SortOrder:  req.SortOrder,
	}

	if err := h.DB.Create(&tier).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tier"})
		return
	}

	c.JSON(http.StatusCreated, TierPayload{
		ID:         tier.ID,
		Code:       tier.Code,
		Name:       tier.Name,
		DailyQuota: tier.DailyQuota,
		Features:   tier.Features,
		PriceMonth: tier.PriceMonth,
		SortOrder:  tier.SortOrder,
	})
}

// UpdateTierRequest is the payload for PUT /api/admin/tiers/:id.
type UpdateTierRequest struct {
	Name       string `json:"name"`
	DailyQuota *int   `json:"dailyQuota"`
	Features   string `json:"features"`
	PriceMonth *int   `json:"priceMonth"`
	SortOrder  *int   `json:"sortOrder"`
}

// UpdateTier updates an existing membership tier.
func (h *AdminTierHandler) UpdateTier(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tier id"})
		return
	}

	var tier model.MembershipTier
	if err := h.DB.First(&tier, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tier not found"})
		return
	}

	var req UpdateTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields
	if req.Name != "" {
		tier.Name = req.Name
	}
	if req.DailyQuota != nil {
		tier.DailyQuota = *req.DailyQuota
	}
	if req.Features != "" {
		tier.Features = req.Features
	}
	if req.PriceMonth != nil {
		tier.PriceMonth = *req.PriceMonth
	}
	if req.SortOrder != nil {
		tier.SortOrder = *req.SortOrder
	}

	if err := h.DB.Save(&tier).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tier"})
		return
	}

	c.JSON(http.StatusOK, TierPayload{
		ID:         tier.ID,
		Code:       tier.Code,
		Name:       tier.Name,
		DailyQuota: tier.DailyQuota,
		Features:   tier.Features,
		PriceMonth: tier.PriceMonth,
		SortOrder:  tier.SortOrder,
	})
}

// DeleteTier removes a membership tier.
// Cannot delete the "free" tier or a tier that has users.
func (h *AdminTierHandler) DeleteTier(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tier id"})
		return
	}

	var tier model.MembershipTier
	if err := h.DB.First(&tier, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tier not found"})
		return
	}

	// Cannot delete free tier
	if tier.Code == model.TierCodeFree {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete the free tier"})
		return
	}

	// Check if tier has users
	var membershipCount int64
	h.DB.Model(&model.UserMembership{}).Where("tier_id = ?", id).Count(&membershipCount)
	if membershipCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete tier with active members"})
		return
	}

	if err := h.DB.Delete(&tier).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete tier"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tier deleted"})
}