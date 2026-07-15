// Package server wires routes onto a Gin engine and applies middleware.
//
// The API lives under /api/* and is JSON. The built React bundle is
// served from "/" in production so the whole product ships as a
// single binary + a single container. In development the frontend
// runs on its own Vite dev server (see frontend/vite.config.ts proxy),
// so only the API is exercised here.
package server

import (
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/handler"
	"predictdestiny/internal/store"
)

// Deps bundles the dependencies a Server needs. Constructed once in
// main and passed in — keeps handlers testable with fakes.
//
// The backend is API-only: the React SPA ships as a separate image and
// is served from a different origin (e.g. Cloudflare Pages), so there
// is no static-asset serving here.
type Deps struct {
	DB       *gorm.DB
	Settings *store.SettingStore
	Gateway  ai.Gateway
}

// New assembles the Gin engine with all routes registered.
func New(deps Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Permissive CORS so the separately-deployed frontend (different
	// origin, e.g. Cloudflare Pages) can call the API. Tighten the
	// allow-list once the production domain is known.
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: false, // must be false when AllowAllOrigins is true
	}))

	// --- handlers ---
	health := &handler.HealthHandler{DB: deps.DB}
	settings := &handler.SettingHandler{Settings: deps.Settings}
	bazi := &handler.BaziHandler{Gateway: deps.Gateway}

	// --- API routes ---
	api := r.Group("/api")
	{
		api.GET("/health", health.Health)
		api.GET("/ready", health.Ready)

		// Dynamic config (admin-only once auth lands in stage 4).
		api.GET("/settings", settings.List)
		api.PUT("/settings", settings.Update)
		api.POST("/settings/reload", settings.Reload)

		// Bazi (stage 1): chart compute is anonymous/free; AI
		// interpret hits the gateway. Auth + quota gating in stage 4.
		api.POST("/bazi/compute", bazi.Compute)
		api.POST("/bazi/interpret", bazi.Interpret)
	}

	// Anything under /api/* that isn't matched returns a JSON 404.
	// Non-/api paths get the same JSON shape — there is no SPA here.
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	return r
}
