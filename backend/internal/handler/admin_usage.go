package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

// AdminAIUsageHandler manages immutable model prices and reads the usage ledger.
type AdminAIUsageHandler struct {
	DB *gorm.DB
}

type createAIPriceRequest struct {
	ProviderID                    uint      `json:"providerId" binding:"required"`
	Model                         string    `json:"model" binding:"required"`
	Version                       string    `json:"version" binding:"required"`
	InputCostMicrosPerMillion     int64     `json:"inputCostMicrosPerMillion"`
	OutputCostMicrosPerMillion    int64     `json:"outputCostMicrosPerMillion"`
	ReasoningCostMicrosPerMillion int64     `json:"reasoningCostMicrosPerMillion"`
	RequestReserveMicros          int64     `json:"requestReserveMicros"`
	EffectiveFrom                 time.Time `json:"effectiveFrom" binding:"required"`
}

type aiPricePayload struct {
	ID                            uint      `json:"id"`
	ProviderID                    uint      `json:"providerId"`
	ProviderName                  string    `json:"providerName"`
	Model                         string    `json:"model"`
	Version                       string    `json:"version"`
	InputCostMicrosPerMillion     int64     `json:"inputCostMicrosPerMillion"`
	OutputCostMicrosPerMillion    int64     `json:"outputCostMicrosPerMillion"`
	ReasoningCostMicrosPerMillion int64     `json:"reasoningCostMicrosPerMillion"`
	RequestReserveMicros          int64     `json:"requestReserveMicros"`
	EffectiveFrom                 time.Time `json:"effectiveFrom"`
	CreatedAt                     time.Time `json:"createdAt"`
}

// ListPrices returns newest effective price versions first.
func (h *AdminAIUsageHandler) ListPrices(c *gin.Context) {
	limit := queryInt(c, "limit", 100, 1, 500)
	offset := queryInt(c, "offset", 0, 0, 1000000)
	var total int64
	if err := h.DB.Model(&model.AIModelPriceVersion{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count prices"})
		return
	}
	var prices []model.AIModelPriceVersion
	err := h.DB.Table("ai_model_price_versions AS prices").
		Select("prices.*").
		Order("prices.effective_from DESC, prices.id DESC").
		Limit(limit).Offset(offset).Scan(&prices).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch prices"})
		return
	}
	var providers []model.AIProvider
	if err := h.DB.Select("id", "name").Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch providers"})
		return
	}
	names := make(map[uint]string, len(providers))
	for _, provider := range providers {
		names[provider.ID] = provider.Name
	}
	rows := make([]aiPricePayload, len(prices))
	for i, price := range prices {
		rows[i] = pricePayload(price, names[price.ProviderID])
	}
	c.JSON(http.StatusOK, gin.H{"prices": rows, "total": total})
}

// CreatePrice appends a price version. Existing versions are never mutated.
func (h *AdminAIUsageHandler) CreatePrice(c *gin.Context) {
	var req createAIPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	req.Version = strings.TrimSpace(req.Version)
	if req.InputCostMicrosPerMillion < 0 || req.OutputCostMicrosPerMillion < 0 || req.ReasoningCostMicrosPerMillion < 0 || req.RequestReserveMicros < 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "price values cannot be negative"})
		return
	}
	var provider model.AIProvider
	if err := h.DB.First(&provider, req.ProviderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "provider not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch provider"})
		}
		return
	}
	if !providerHasModel(provider.Models, req.Model) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "model is not in the provider catalog"})
		return
	}
	price := model.AIModelPriceVersion{
		ProviderID:                    req.ProviderID,
		Model:                         req.Model,
		Version:                       req.Version,
		InputCostMicrosPerMillion:     req.InputCostMicrosPerMillion,
		OutputCostMicrosPerMillion:    req.OutputCostMicrosPerMillion,
		ReasoningCostMicrosPerMillion: req.ReasoningCostMicrosPerMillion,
		RequestReserveMicros:          req.RequestReserveMicros,
		EffectiveFrom:                 req.EffectiveFrom.UTC(),
	}
	if err := h.DB.Create(&price).Error; err != nil {
		var existing int64
		h.DB.Model(&model.AIModelPriceVersion{}).
			Where("provider_id = ? AND model = ? AND version = ?", price.ProviderID, price.Model, price.Version).
			Count(&existing)
		if existing > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "price version already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create price"})
		return
	}
	c.JSON(http.StatusCreated, pricePayload(price, provider.Name))
}

type usageTotals struct {
	Requests            int64 `json:"requests"`
	Succeeded           int64 `json:"succeeded"`
	Failed              int64 `json:"failed"`
	Cancelled           int64 `json:"cancelled"`
	Unpriced            int64 `json:"unpriced"`
	PromptTokens        int64 `json:"promptTokens"`
	CompletionTokens    int64 `json:"completionTokens"`
	ReasoningTokens     int64 `json:"reasoningTokens"`
	TotalTokens         int64 `json:"totalTokens"`
	EstimatedCostMicros int64 `json:"estimatedCostMicros"`
	ActualCostMicros    int64 `json:"actualCostMicros"`
	ActualCostCount     int64 `json:"actualCostCount"`
}

type usageGroup struct {
	ProviderID   *uint  `json:"providerId"`
	ProviderName string `json:"providerName"`
	Model        string `json:"model"`
	usageTotals
}

// UsageSummary aggregates ledger rows for a bounded UTC date range.
func (h *AdminAIUsageHandler) UsageSummary(c *gin.Context) {
	now := time.Now().UTC()
	from, err := queryDate(c, "from", now.AddDate(0, 0, -29))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from must use YYYY-MM-DD"})
		return
	}
	to, err := queryDate(c, "to", now)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to must use YYYY-MM-DD"})
		return
	}
	if to.Before(from) || to.Sub(from) > 366*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date range must be ordered and no longer than 366 days"})
		return
	}

	base := h.DB.Model(&model.AIUsageLedger{}).
		Where("started_at >= ? AND started_at < ?", from, to.AddDate(0, 0, 1))
	if raw := c.Query("providerId"); raw != "" {
		id, parseErr := strconv.ParseUint(raw, 10, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid providerId"})
			return
		}
		base = base.Where("provider_id = ?", uint(id))
	}
	if selectedModel := strings.TrimSpace(c.Query("model")); selectedModel != "" {
		base = base.Where("model = ?", selectedModel)
	}
	aggregates := `COUNT(*) AS requests,
		COALESCE(SUM(CASE WHEN status = 'succeeded' THEN 1 ELSE 0 END), 0) AS succeeded,
		COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) AS failed,
		COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0) AS cancelled,
		COALESCE(SUM(CASE WHEN pricing_status = 'unpriced' THEN 1 ELSE 0 END), 0) AS unpriced,
		COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
		COALESCE(SUM(reasoning_tokens), 0) AS reasoning_tokens,
		COALESCE(SUM(total_tokens), 0) AS total_tokens,
		COALESCE(SUM(estimated_cost_micros), 0) AS estimated_cost_micros,
		COALESCE(SUM(actual_cost_micros), 0) AS actual_cost_micros,
		COALESCE(SUM(CASE WHEN actual_cost_micros IS NOT NULL THEN 1 ELSE 0 END), 0) AS actual_cost_count`
	var totals usageTotals
	if err := base.Select(aggregates).Scan(&totals).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to summarize usage"})
		return
	}
	var groups []usageGroup
	if err := base.Select("provider_id, provider_name, model, " + aggregates).
		Group("provider_id, provider_name, model").
		Order("estimated_cost_micros DESC, requests DESC").
		Scan(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to group usage"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"),
		"totals": totals, "groups": groups,
	})
}

func providerHasModel(raw, selected string) bool {
	var entries []struct {
		ID string `json:"id"`
	}
	if json.Unmarshal([]byte(raw), &entries) != nil {
		return false
	}
	for _, entry := range entries {
		if entry.ID == selected {
			return true
		}
	}
	return false
}

func pricePayload(price model.AIModelPriceVersion, providerName string) aiPricePayload {
	return aiPricePayload{
		ID: price.ID, ProviderID: price.ProviderID, ProviderName: providerName,
		Model: price.Model, Version: price.Version,
		InputCostMicrosPerMillion:     price.InputCostMicrosPerMillion,
		OutputCostMicrosPerMillion:    price.OutputCostMicrosPerMillion,
		ReasoningCostMicrosPerMillion: price.ReasoningCostMicrosPerMillion,
		RequestReserveMicros:          price.RequestReserveMicros,
		EffectiveFrom:                 price.EffectiveFrom, CreatedAt: price.CreatedAt,
	}
}

func queryDate(c *gin.Context, name string, fallback time.Time) (time.Time, error) {
	raw := c.Query(name)
	if raw == "" {
		raw = fallback.Format("2006-01-02")
	}
	return time.Parse("2006-01-02", raw)
}

func queryInt(c *gin.Context, name string, fallback, min, max int) int {
	value, err := strconv.Atoi(c.Query(name))
	if err != nil || value < min {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}
