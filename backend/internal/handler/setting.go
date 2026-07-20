package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"predictdestiny/internal/store"
)

const maskedSecret = "********"

// SettingHandler exposes the dynamic-config table to the admin panel.
//
// For now the routes are not yet admin-gated — that wire-up happens
// in stage 4 when authentication lands. The shape is fixed here so
// the frontend can be built against a stable contract.
type SettingHandler struct {
	Settings *store.SettingStore
}

// List returns all settings, grouped/sorted as stored.
// GET /api/settings
func (h *SettingHandler) List(c *gin.Context) {
	rows, err := h.Settings.All()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range rows {
		if rows[i].Kind == "password" && rows[i].Value != "" {
			rows[i].Value = maskedSecret
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// Update applies one or more key/value edits atomically.
// PUT /api/settings  body: {"items": {"ai.api_key": "sk-...", ...}}
//
// Password-kind values returned by List are masked; the admin UI sends
// the real value only when it actually changes, so this endpoint just
// writes whatever it receives.
func (h *SettingHandler) Update(c *gin.Context) {
	var body struct {
		Items map[string]string `json:"items"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(body.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no items"})
		return
	}
	// The masked value means the admin left an existing secret unchanged.
	for key, value := range body.Items {
		if value == maskedSecret {
			delete(body.Items, key)
		}
	}
	if len(body.Items) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	if err := h.Settings.SetMany(body.Items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Reload forces the in-memory cache to refresh from the DB. Handy if
// an operator edits the table directly.
// POST /api/settings/reload
func (h *SettingHandler) Reload(c *gin.Context) {
	if err := h.Settings.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
