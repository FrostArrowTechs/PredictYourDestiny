// Package handler groups all HTTP handlers.
//
// Each domain (health, setting, bazi, …) lives in its own file and
// exposes a struct that closes over its dependencies. Handlers stay
// thin: parse request → call store/engine → render JSON.
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/version"
)

// HealthHandler exposes liveness / readiness checks.
type HealthHandler struct {
	DB *gorm.DB
}

// Health reports process liveness. It always returns 200 if the
// process is up — it does NOT probe dependencies.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": version.Version,
		"time":    time.Now().Format(time.RFC3339),
	})
}

// Ready reports readiness: it fails if the database is unreachable,
// which lets the load balancer stop routing traffic during an outage.
func (h *HealthHandler) Ready(c *gin.Context) {
	sqlDB, err := h.DB.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_error"})
		return
	}
	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unreachable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
