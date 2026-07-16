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
	"predictdestiny/internal/auth"
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
	authH := &handler.AuthHandler{DB: deps.DB}
	record := &handler.RecordHandler{DB: deps.DB}
	quota := &handler.QuotaHandler{DB: deps.DB}
	adminUser := &handler.AdminUserHandler{DB: deps.DB}
	adminProvider := &handler.AdminProviderHandler{DB: deps.DB}
	adminTier := &handler.AdminTierHandler{DB: deps.DB}
	bazi := &handler.BaziHandler{Gateway: deps.Gateway}
	dream := &handler.DreamHandler{Gateway: deps.Gateway, DB: deps.DB}
	huangli := &handler.HuangliHandler{Gateway: deps.Gateway}
	zodiac := &handler.ZodiacHandler{Gateway: deps.Gateway}
	compatibility := &handler.CompatibilityHandler{Gateway: deps.Gateway}
	weighbone := &handler.WeighboneHandler{Gateway: deps.Gateway}
	divination := &handler.DivinationHandler{Gateway: deps.Gateway, DB: deps.DB}
	plumflower := &handler.PlumFlowerHandler{Gateway: deps.Gateway}
	name := &handler.NameHandler{Gateway: deps.Gateway, DB: deps.DB}
	astrology := &handler.AstrologyHandler{Gateway: deps.Gateway}
	constellation := &handler.ConstellationHandler{Gateway: deps.Gateway}
	tarot := &handler.TarotHandler{Gateway: deps.Gateway, DB: deps.DB}
	ziwei := &handler.ZiweiHandler{Gateway: deps.Gateway}

	// --- API routes ---
	api := r.Group("/api")
	{
		api.GET("/health", health.Health)
		api.GET("/ready", health.Ready)

		// Auth endpoints (public)
		api.POST("/auth/register", authH.Register)
		api.POST("/auth/login", authH.Login)
		api.GET("/auth/me", auth.AuthRequired(), authH.Me)

		// User data endpoints (require auth)
		api.GET("/records", auth.AuthRequired(), record.List)
		api.POST("/records", auth.AuthRequired(), record.Create)
		api.GET("/records/:id", auth.AuthRequired(), record.Get)
		api.DELETE("/records/:id", auth.AuthRequired(), record.Delete)
		api.GET("/quota", auth.AuthRequired(), quota.Get)

		// Dynamic config (admin-only).
		api.GET("/settings", settings.List)
		api.PUT("/settings", auth.AdminRequired(), settings.Update)
		api.POST("/settings/reload", auth.AdminRequired(), settings.Reload)

		// Admin endpoints (require admin role)
		admin := api.Group("/admin", auth.AdminRequired())
		{
			// User management
			admin.GET("/users", adminUser.ListUsers)
			admin.GET("/users/:id", adminUser.GetUser)
			admin.PUT("/users/:id/role", adminUser.UpdateUserRole)
			admin.PUT("/users/:id/tier", adminUser.UpdateUserTier)

			// AI Provider management
			admin.GET("/providers", adminProvider.ListProviders)
			admin.POST("/providers", adminProvider.CreateProvider)
			admin.PUT("/providers/:id", adminProvider.UpdateProvider)
			admin.DELETE("/providers/:id", adminProvider.DeleteProvider)
			admin.POST("/providers/:id/default", adminProvider.SetDefaultProvider)

			// Membership tier management
			admin.GET("/tiers", adminTier.ListTiers)
			admin.POST("/tiers", adminTier.CreateTier)
			admin.PUT("/tiers/:id", adminTier.UpdateTier)
			admin.DELETE("/tiers/:id", adminTier.DeleteTier)
		}

		// Bazi (stage 1): chart compute is anonymous/free; AI
		// interpret hits the gateway. Auth + quota gating in stage 4.
		api.POST("/bazi/compute", bazi.Compute)
		api.POST("/bazi/interpret", bazi.Interpret)

		// Dream (stage 2): keyword search + AI interpretation.
		api.POST("/dream/compute", dream.Compute)
		api.POST("/dream/interpret", dream.Interpret)

		// Huangli (stage 2): calendar data + AI advice.
		api.POST("/huangli/compute", huangli.Compute)
		api.POST("/huangli/interpret", huangli.Interpret)

		// Zodiac (stage 2): fortune calculation + AI interpretation.
		api.POST("/zodiac/compute", zodiac.Compute)
		api.POST("/zodiac/interpret", zodiac.Interpret)

		// Compatibility (stage 2): match analysis + AI interpretation.
		api.POST("/compatibility/compute", compatibility.Compute)
		api.POST("/compatibility/interpret", compatibility.Interpret)

		// Weighbone (stage 3 batch 1): bone weight fortune.
		api.POST("/weighbone/compute", weighbone.Compute)
		api.POST("/weighbone/interpret", weighbone.Interpret)

		// Divination (stage 3 batch 1): draw divination stick + interpret.
		api.POST("/divination/compute", divination.Compute)
		api.POST("/divination/interpret", divination.Interpret)

			// Plum flower (stage 3 batch 1): hexagram divination.
			api.POST("/plumflower/compute", plumflower.Compute)
			api.POST("/plumflower/interpret", plumflower.Interpret)

			// Name (stage 3 batch 2): Five格 name analysis.
			api.POST("/name/compute", name.Compute)
			api.POST("/name/interpret", name.Interpret)

			// Astrology (stage 3 batch 2): Western natal chart.
			api.POST("/astrology/compute", astrology.Compute)
			api.POST("/astrology/interpret", astrology.Interpret)

			// Constellation (stage 3 batch 3): sun-sign daily fortune.
			api.POST("/constellation/compute", constellation.Compute)
			api.POST("/constellation/interpret", constellation.Interpret)

			// Tarot (stage 3 batch 3): card draw + spread interpretation.
			api.POST("/tarot/draw", tarot.Draw)
			api.POST("/tarot/compute", tarot.Draw)
			api.POST("/tarot/interpret", tarot.Interpret)

			// Ziwei (stage 3 batch 3): 紫微斗数 natal chart.
			api.POST("/ziwei/compute", ziwei.Compute)
			api.POST("/ziwei/interpret", ziwei.Interpret)
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
