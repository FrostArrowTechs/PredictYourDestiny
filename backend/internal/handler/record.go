package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/auth"
	"predictdestiny/internal/model"
)

// RecordHandler handles user's fortune record endpoints.
type RecordHandler struct {
	DB *gorm.DB
}

// RecordResponse is the API response for a single record.
type RecordResponse struct {
	ID              uint   `json:"id"`
	Kind            string `json:"kind"`
	Title           string `json:"title"`
	InputJSON       string `json:"inputJson"`
	ResultJSON      string `json:"resultJson"`
	ServerGenerated bool   `json:"serverGenerated"`
	Note            string `json:"note"`
	CreatedAt       string `json:"createdAt"`
}

// List returns the authenticated user's saved records.
// Supports pagination via ?page=1&limit=20 and filtering via ?kind=bazi.
func (h *RecordHandler) List(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := h.DB.Model(&model.FortuneRecord{}).Where("user_id = ?", userID)

	// Optional kind filter
	if kind := c.Query("kind"); kind != "" {
		query = query.Where("kind = ?", kind)
	}

	var total int64
	query.Count(&total)

	var records []model.FortuneRecord
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch records"})
		return
	}

	res := make([]RecordResponse, len(records))
	for i, r := range records {
		res[i] = RecordResponse{
			ID:              r.ID,
			Kind:            r.Kind,
			Title:           r.Title,
			InputJSON:       r.InputJSON,
			ResultJSON:      r.ResultJSON,
			ServerGenerated: r.ServerGenerated,
			Note:            r.Note,
			CreatedAt:       r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"records": res,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// Get returns a single record by ID (must belong to the authenticated user).
func (h *RecordHandler) Get(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid record id"})
		return
	}

	var record model.FortuneRecord
	if err := h.DB.Where("id = ? AND user_id = ?", id, userID).First(&record).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
		return
	}

	c.JSON(http.StatusOK, RecordResponse{
		ID:              record.ID,
		Kind:            record.Kind,
		Title:           record.Title,
		InputJSON:       record.InputJSON,
		ResultJSON:      record.ResultJSON,
		ServerGenerated: record.ServerGenerated,
		Note:            record.Note,
		CreatedAt:       record.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// writeComputedResult persists only the input accepted by a compute handler and
// the result produced by the server. Anonymous calculations remain ephemeral.
func writeComputedResult(c *gin.Context, db *gorm.DB, kind string, input, result any) {
	userID := auth.GetUserID(c)
	if userID != 0 && auth.HistoryPersistenceEnabled(c) {
		inputJSON, inputErr := json.Marshal(input)
		resultJSON, resultErr := json.Marshal(result)
		if inputErr != nil || resultErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode server result"})
			return
		}
		now := time.Now()
		record := model.FortuneRecord{
			UserID: userID, Kind: kind,
			Title:     kind + " · " + now.Format("2006-01-02 15:04"),
			InputJSON: string(inputJSON), ResultJSON: string(resultJSON),
			ServerGenerated: true,
		}
		if db == nil || db.Create(&record).Error != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "history persistence unavailable"})
			return
		}
		c.Header("X-Fortune-Record-ID", strconv.FormatUint(uint64(record.ID), 10))
	}
	c.JSON(http.StatusOK, result)
}

// Delete removes a record (must belong to the authenticated user).
func (h *RecordHandler) Delete(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid record id"})
		return
	}

	result := h.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.FortuneRecord{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete record"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "record deleted"})
}
