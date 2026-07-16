package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

// AdminProviderHandler handles AI provider management endpoints.
type AdminProviderHandler struct {
	DB *gorm.DB
}

// ProviderPayload is the API response for a provider.
type ProviderPayload struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"baseUrl"`
	Models    string `json:"models"`
	IsDefault bool   `json:"isDefault"`
	IsEnabled bool   `json:"isEnabled"`
	SortOrder int    `json:"sortOrder"`
}

// ListProviders returns all AI providers.
func (h *AdminProviderHandler) ListProviders(c *gin.Context) {
	var providers []model.AIProvider
	if err := h.DB.Order("sort_order, id").Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch providers"})
		return
	}

	payload := make([]ProviderPayload, len(providers))
	for i, p := range providers {
		payload[i] = ProviderPayload{
			ID:        p.ID,
			Name:      p.Name,
			BaseURL:   p.BaseURL,
			Models:    p.Models,
			IsDefault: p.IsDefault,
			IsEnabled: p.IsEnabled,
			SortOrder: p.SortOrder,
		}
	}

	c.JSON(http.StatusOK, gin.H{"providers": payload})
}

// CreateProviderRequest is the payload for POST /api/admin/providers.
type CreateProviderRequest struct {
	Name      string `json:"name" binding:"required"`
	BaseURL   string `json:"baseUrl" binding:"required"`
	APIKey    string `json:"apiKey"`
	Models    string `json:"models"`
	IsDefault bool   `json:"isDefault"`
	IsEnabled bool   `json:"isEnabled"`
	SortOrder int    `json:"sortOrder"`
}

// CreateProvider creates a new AI provider.
func (h *AdminProviderHandler) CreateProvider(c *gin.Context) {
	var req CreateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider := model.AIProvider{
		Name:      req.Name,
		BaseURL:   req.BaseURL,
		APIKey:    req.APIKey,
		Models:    req.Models,
		IsDefault: req.IsDefault,
		IsEnabled: req.IsEnabled,
		SortOrder: req.SortOrder,
	}

	// If this is set as default, unset other defaults
	if provider.IsDefault {
		h.DB.Model(&model.AIProvider{}).Where("is_default = ?", true).Update("is_default", false)
	}

	if err := h.DB.Create(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create provider"})
		return
	}

	c.JSON(http.StatusCreated, ProviderPayload{
		ID:        provider.ID,
		Name:      provider.Name,
		BaseURL:   provider.BaseURL,
		Models:    provider.Models,
		IsDefault: provider.IsDefault,
		IsEnabled: provider.IsEnabled,
		SortOrder: provider.SortOrder,
	})
}

// UpdateProviderRequest is the payload for PUT /api/admin/providers/:id.
type UpdateProviderRequest struct {
	Name      string `json:"name"`
	BaseURL   string `json:"baseUrl"`
	APIKey    string `json:"apiKey"`
	Models    string `json:"models"`
	IsDefault *bool  `json:"isDefault"`
	IsEnabled *bool  `json:"isEnabled"`
	SortOrder *int   `json:"sortOrder"`
}

// UpdateProvider updates an existing AI provider.
func (h *AdminProviderHandler) UpdateProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	var provider model.AIProvider
	if err := h.DB.First(&provider, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	var req UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields
	if req.Name != "" {
		provider.Name = req.Name
	}
	if req.BaseURL != "" {
		provider.BaseURL = req.BaseURL
	}
	if req.APIKey != "" {
		provider.APIKey = req.APIKey
	}
	if req.Models != "" {
		provider.Models = req.Models
	}
	if req.IsDefault != nil {
		if *req.IsDefault {
			// Unset other defaults first
			h.DB.Model(&model.AIProvider{}).Where("is_default = ? AND id != ?", true, id).Update("is_default", false)
		}
		provider.IsDefault = *req.IsDefault
	}
	if req.IsEnabled != nil {
		provider.IsEnabled = *req.IsEnabled
	}
	if req.SortOrder != nil {
		provider.SortOrder = *req.SortOrder
	}

	if err := h.DB.Save(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update provider"})
		return
	}

	c.JSON(http.StatusOK, ProviderPayload{
		ID:        provider.ID,
		Name:      provider.Name,
		BaseURL:   provider.BaseURL,
		Models:    provider.Models,
		IsDefault: provider.IsDefault,
		IsEnabled: provider.IsEnabled,
		SortOrder: provider.SortOrder,
	})
}

// DeleteProvider removes an AI provider.
func (h *AdminProviderHandler) DeleteProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	result := h.DB.Delete(&model.AIProvider{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete provider"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "provider deleted"})
}

// SetDefaultProvider sets a provider as the default.
func (h *AdminProviderHandler) SetDefaultProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	var provider model.AIProvider
	if err := h.DB.First(&provider, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	// Unset all defaults
	h.DB.Model(&model.AIProvider{}).Where("is_default = ?", true).Update("is_default", false)

	// Set this one as default
	provider.IsDefault = true
	h.DB.Save(&provider)

	c.JSON(http.StatusOK, gin.H{"message": "default provider set"})
}