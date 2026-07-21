package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/model"
	"predictdestiny/internal/secret"
)

// AdminProviderHandler handles AI provider management endpoints.
type AdminProviderHandler struct {
	DB     *gorm.DB
	Cipher *secret.Cipher
}

// ProviderPayload is the API response for a provider.
type ProviderPayload struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"baseUrl"`
	APIKey    string `json:"apiKey"`
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
			APIKey:    maskProviderKey(p.APIKey),
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
		Models:    req.Models,
		IsDefault: req.IsDefault,
		IsEnabled: req.IsEnabled,
		SortOrder: req.SortOrder,
	}
	if req.APIKey != "" && req.APIKey != maskedSecret {
		encrypted, err := h.Cipher.Encrypt(req.APIKey)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider key encryption is not configured"})
			return
		}
		provider.APIKey = encrypted
	}
	if provider.IsDefault && !provider.IsEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "default provider must be enabled"})
		return
	}
	if err := ai.ValidateProviderBaseURL(provider.BaseURL, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if provider.IsDefault {
			if err := tx.Model(&model.AIProvider{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&provider).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create provider"})
		return
	}

	c.JSON(http.StatusCreated, ProviderPayload{
		ID:        provider.ID,
		Name:      provider.Name,
		BaseURL:   provider.BaseURL,
		APIKey:    maskProviderKey(provider.APIKey),
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
	if req.APIKey != "" && req.APIKey != maskedSecret {
		encrypted, err := h.Cipher.Encrypt(req.APIKey)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider key encryption is not configured"})
			return
		}
		provider.APIKey = encrypted
	}
	if req.Models != "" {
		provider.Models = req.Models
	}
	if req.IsDefault != nil {
		if provider.IsDefault && !*req.IsDefault {
			c.JSON(http.StatusConflict, gin.H{"error": "select another default instead of clearing it"})
			return
		}
		provider.IsDefault = *req.IsDefault
	}
	if req.IsEnabled != nil {
		if provider.IsDefault && !*req.IsEnabled {
			c.JSON(http.StatusConflict, gin.H{"error": "select another default before disabling this provider"})
			return
		}
		provider.IsEnabled = *req.IsEnabled
	}
	if req.SortOrder != nil {
		provider.SortOrder = *req.SortOrder
	}

	if provider.IsDefault && !provider.IsEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "default provider must be enabled"})
		return
	}
	if err := ai.ValidateProviderBaseURL(provider.BaseURL, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if provider.IsDefault {
			if err := tx.Model(&model.AIProvider{}).Where("is_default = ? AND id != ?", true, id).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(&provider).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update provider"})
		return
	}

	c.JSON(http.StatusOK, ProviderPayload{
		ID:        provider.ID,
		Name:      provider.Name,
		BaseURL:   provider.BaseURL,
		APIKey:    maskProviderKey(provider.APIKey),
		Models:    provider.Models,
		IsDefault: provider.IsDefault,
		IsEnabled: provider.IsEnabled,
		SortOrder: provider.SortOrder,
	})
}

func maskProviderKey(value string) string {
	if value == "" {
		return ""
	}
	return maskedSecret
}

// DeleteProvider removes an AI provider.
func (h *AdminProviderHandler) DeleteProvider(c *gin.Context) {
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
	if provider.IsDefault {
		c.JSON(http.StatusConflict, gin.H{"error": "select another default before deleting this provider"})
		return
	}

	result := h.DB.Delete(&provider)
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
	if !provider.IsEnabled {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot make a disabled provider the default"})
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AIProvider{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&provider).Update("is_default", true).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set default provider"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "default provider set"})
}

// CheckProviderHealth verifies connectivity and credentials through the
// provider's /models endpoint without creating a billable completion.
func (h *AdminProviderHandler) CheckProviderHealth(c *gin.Context) {
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
	apiKey, err := h.Cipher.Decrypt(provider.APIKey)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider key is unavailable"})
		return
	}
	duration, err := ai.CheckProviderHealth(c.Request.Context(), provider.BaseURL, apiKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider health check failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy", "latencyMs": duration.Milliseconds()})
}
