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
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/auth"
	"predictdestiny/internal/handler"
	"predictdestiny/internal/secret"
	"predictdestiny/internal/store"
)

// Deps bundles the dependencies a Server needs. Constructed once in
// main and passed in — keeps handlers testable with fakes.
//
// The backend is API-only: the React SPA ships as a separate image and
// is served from a different origin (e.g. Cloudflare Pages), so there
// is no static-asset serving here.
type Deps struct {
	DB             *gorm.DB
	Settings       *store.SettingStore
	Gateway        ai.Gateway
	SecretCipher   *secret.Cipher
	AllowedOrigins []string
	Production     bool
}

// New assembles the Gin engine with all routes registered.
func New(deps Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(requestIDMiddleware(), securityHeaders(deps.Production), gin.Logger(), gin.Recovery())

	allowedOrigins := deps.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{
			"http://localhost:5173", "http://localhost:5174",
			"http://127.0.0.1:5173", "http://127.0.0.1:5174",
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID", "X-Fortune-Record-ID"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Basic abuse protection. Authentication endpoints are keyed by IP;
	// authenticated AI endpoints are keyed by user ID so changing IP cannot
	// multiply a user's allowance. Quotas remain the billing authority.
	authLimiter := newFixedWindowLimiter(10, time.Minute)
	aiLimiter := newFixedWindowLimiter(30, time.Minute)
	authRateLimit := authLimiter.middleware(clientIPKey("auth"))
	aiRateLimit := aiLimiter.middleware(userOrIPKey("ai"))

	// --- handlers ---
	health := &handler.HealthHandler{DB: deps.DB}
	settings := &handler.SettingHandler{Settings: deps.Settings}
	authH := &handler.AuthHandler{DB: deps.DB}
	account := &handler.AccountHandler{DB: deps.DB}
	record := &handler.RecordHandler{DB: deps.DB}
	quota := &handler.QuotaHandler{DB: deps.DB}
	entitlements := &handler.EntitlementHandler{DB: deps.DB, Gateway: deps.Gateway}
	adminUser := &handler.AdminUserHandler{DB: deps.DB}
	adminProvider := &handler.AdminProviderHandler{DB: deps.DB, Cipher: deps.SecretCipher}
	adminTier := &handler.AdminTierHandler{DB: deps.DB}
	adminUsage := &handler.AdminAIUsageHandler{DB: deps.DB}
	bazi := &handler.BaziHandler{Gateway: deps.Gateway, DB: deps.DB}
	dream := &handler.DreamHandler{Gateway: deps.Gateway, DB: deps.DB}
	huangli := &handler.HuangliHandler{Gateway: deps.Gateway, DB: deps.DB}
	zodiac := &handler.ZodiacHandler{Gateway: deps.Gateway, DB: deps.DB}
	compatibility := &handler.CompatibilityHandler{Gateway: deps.Gateway, DB: deps.DB}
	weighbone := &handler.WeighboneHandler{Gateway: deps.Gateway, DB: deps.DB}
	divination := &handler.DivinationHandler{Gateway: deps.Gateway, DB: deps.DB}
	plumflower := &handler.PlumFlowerHandler{Gateway: deps.Gateway, DB: deps.DB}
	name := &handler.NameHandler{Gateway: deps.Gateway, DB: deps.DB}
	astrology := &handler.AstrologyHandler{Gateway: deps.Gateway, DB: deps.DB}
	constellation := &handler.ConstellationHandler{Gateway: deps.Gateway, DB: deps.DB}
	tarot := &handler.TarotHandler{Gateway: deps.Gateway, DB: deps.DB}
	ziwei := &handler.ZiweiHandler{Gateway: deps.Gateway, DB: deps.DB}

	// --- API routes ---
	api := r.Group("/api")
	{
		api.GET("/health", health.Health)
		api.GET("/ready", health.Ready)

		// Auth endpoints (public)
		api.POST("/auth/register", authRateLimit, authH.Register)
		api.POST("/auth/login", authRateLimit, authH.Login)
		api.GET("/auth/me", auth.AuthRequired(deps.DB), authH.Me)
		api.GET("/account/export", auth.AuthRequired(deps.DB), account.Export)
		api.DELETE("/account", auth.AuthRequired(deps.DB), account.Delete)

		// User data endpoints (require auth)
		api.GET("/records", auth.AuthRequired(deps.DB), record.List)
		api.DELETE("/records", auth.AuthRequired(deps.DB), account.ClearHistory)
		api.GET("/records/:id", auth.AuthRequired(deps.DB), record.Get)
		api.DELETE("/records/:id", auth.AuthRequired(deps.DB), record.Delete)
		api.GET("/quota", auth.AuthRequired(deps.DB), quota.Get)
		api.GET("/entitlements", auth.AuthRequired(deps.DB), entitlements.Get)

		// Dynamic config (admin-only).
		api.GET("/settings", auth.AdminRequired(deps.DB), settings.List)
		api.PUT("/settings", auth.AdminRequired(deps.DB), settings.Update)
		api.POST("/settings/reload", auth.AdminRequired(deps.DB), settings.Reload)

		// Admin endpoints (require admin role)
		admin := api.Group("/admin", auth.AdminRequired(deps.DB))
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
			admin.POST("/providers/:id/health", adminProvider.CheckProviderHealth)

			// Membership tier management
			admin.GET("/tiers", adminTier.ListTiers)
			admin.POST("/tiers", adminTier.CreateTier)
			admin.PUT("/tiers/:id", adminTier.UpdateTier)
			admin.DELETE("/tiers/:id", adminTier.DeleteTier)

			// Immutable AI prices and cost observability.
			admin.GET("/ai/prices", adminUsage.ListPrices)
			admin.POST("/ai/prices", adminUsage.CreatePrice)
			admin.GET("/ai/usage/summary", adminUsage.UsageSummary)
		}

		// Bazi (stage 1): chart compute is anonymous/free; AI
		// interpret hits the gateway. Auth + quota gating in stage 4.
		api.POST("/bazi/compute", auth.OptionalAuth(deps.DB), bazi.Compute)
		api.POST("/bazi/interpret", auth.AuthRequired(deps.DB), aiRateLimit, bazi.Interpret)

		// Dream (stage 2): keyword search + AI interpretation.
		api.POST("/dream/compute", auth.OptionalAuth(deps.DB), dream.Compute)
		api.POST("/dream/interpret", auth.AuthRequired(deps.DB), aiRateLimit, dream.Interpret)

		// Huangli (stage 2): calendar data + AI advice.
		api.POST("/huangli/compute", auth.OptionalAuth(deps.DB), huangli.Compute)
		api.POST("/huangli/interpret", auth.AuthRequired(deps.DB), aiRateLimit, huangli.Interpret)

		// Zodiac (stage 2): fortune calculation + AI interpretation.
		api.POST("/zodiac/compute", auth.OptionalAuth(deps.DB), zodiac.Compute)
		api.POST("/zodiac/interpret", auth.AuthRequired(deps.DB), aiRateLimit, zodiac.Interpret)

		// Compatibility (stage 2): match analysis + AI interpretation.
		api.POST("/compatibility/compute", auth.OptionalAuth(deps.DB), compatibility.Compute)
		api.POST("/compatibility/interpret", auth.AuthRequired(deps.DB), aiRateLimit, compatibility.Interpret)

		// Weighbone (stage 3 batch 1): bone weight fortune.
		api.POST("/weighbone/compute", auth.OptionalAuth(deps.DB), weighbone.Compute)
		api.POST("/weighbone/interpret", auth.AuthRequired(deps.DB), aiRateLimit, weighbone.Interpret)

		// Divination (stage 3 batch 1): draw divination stick + interpret.
		api.POST("/divination/compute", auth.OptionalAuth(deps.DB), divination.Compute)
		api.POST("/divination/interpret", auth.AuthRequired(deps.DB), aiRateLimit, divination.Interpret)

		// Plum flower (stage 3 batch 1): hexagram divination.
		api.POST("/plumflower/compute", auth.OptionalAuth(deps.DB), plumflower.Compute)
		api.POST("/plumflower/interpret", auth.AuthRequired(deps.DB), aiRateLimit, plumflower.Interpret)

		// Name (stage 3 batch 2): Five格 name analysis.
		api.POST("/name/compute", auth.OptionalAuth(deps.DB), name.Compute)
		api.POST("/name/interpret", auth.AuthRequired(deps.DB), aiRateLimit, name.Interpret)

		// Astrology (stage 3 batch 2): Western natal chart.
		api.POST("/astrology/compute", auth.OptionalAuth(deps.DB), astrology.Compute)
		api.POST("/astrology/interpret", auth.AuthRequired(deps.DB), aiRateLimit, astrology.Interpret)

		// Constellation (stage 3 batch 3): sun-sign daily fortune.
		api.POST("/constellation/compute", auth.OptionalAuth(deps.DB), constellation.Compute)
		api.POST("/constellation/interpret", auth.AuthRequired(deps.DB), aiRateLimit, constellation.Interpret)

		// Tarot (stage 3 batch 3): card draw + spread interpretation.
		api.POST("/tarot/draw", auth.OptionalAuth(deps.DB), tarot.Draw)
		api.POST("/tarot/compute", auth.OptionalAuth(deps.DB), tarot.Draw)
		api.POST("/tarot/interpret", auth.AuthRequired(deps.DB), aiRateLimit, tarot.Interpret)

		// Ziwei (stage 3 batch 3): 紫微斗数 natal chart.
		api.POST("/ziwei/compute", auth.OptionalAuth(deps.DB), ziwei.Compute)
		api.POST("/ziwei/interpret", auth.AuthRequired(deps.DB), aiRateLimit, ziwei.Interpret)
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
