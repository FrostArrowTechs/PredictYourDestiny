// Command server is the single binary that runs PredictYourDestiny.
//
// Boot sequence:
//
//  1. Load bootstrap config (DATABASE_URL, SERVER_ADDR, …) from env/.env
//  2. Open the GORM connection to PostgreSQL
//  3. AutoMigrate every model — add tables/columns as structs evolve
//  4. Seed default settings into the settings table (idempotent)
//  5. Build the Gin engine and start serving HTTP
//
// Everything beyond the database URL is dynamic config: it lives in
// the settings table and can be changed at runtime via the admin API.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/6tail/lunar-go/calendar" // sanity-check the dep is resolvable
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/config"
	"predictdestiny/internal/model"
	"predictdestiny/internal/server"
	"predictdestiny/internal/store"
	"predictdestiny/internal/version"
)

func main() {
	log.Printf("PredictYourDestiny %s starting…", version.Version)

	// 1) bootstrap config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// 2) database
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("raw db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// 3) auto-migrate
	if err := db.AutoMigrate(
		&model.User{},
		&model.Setting{},
		&model.FortuneRecord{},
		&model.ChatHistory{},
		&model.UsageQuota{},
	); err != nil {
		log.Fatalf("automigrate: %v", err)
	}
	log.Println("database ready, schema migrated")

	// 4) seed default settings
	settingStore, err := store.NewSettingStore(db, defaultSettings())
	if err != nil {
		log.Fatalf("seed settings: %v", err)
	}

	// AI gateway reads base_url / api_key / model list from the
	// settings table, so it's ready the moment those are configured
	// via the admin API — no restart needed.
	gateway := ai.NewOpenAIGateway(settingStore)

	// Touch lunar-go so an import error surfaces here, not deep in a
	// request. (Real usage begins in stage 1 / bazi.)
	_ = calendar.NewSolar(2000, 1, 1, 12, 0, 0)

	// 5) serve
	srv := &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           server.New(server.Deps{DB: db, Settings: settingStore, Gateway: gateway}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.ServerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
	log.Println("bye")
}

// defaultSettings returns the seed rows for the settings table.
// Existing values are never overwritten, so admin edits persist across
// restarts; these defaults only fill in keys that are missing.
func defaultSettings() []model.Setting {
	return []model.Setting{
		// --- AI gateway (New-API / OpenAI-compatible) ---
		{
			Key: model.SettingAIBaseURL, Value: "",
			Kind: model.SettingKindText, Group: model.SettingGroupAI,
			Label: "AI 网关地址 (Base URL)", Hint: "OpenAI 兼容接口，如 https://newapi.example.com/v1",
			SortOrder: 10,
		},
		{
			Key: model.SettingAIAPIKey, Value: "",
			Kind: model.SettingKindPassword, Group: model.SettingGroupAI,
			Label: "AI 网关密钥 (API Key)", Hint: "前端仅显示掩码，保存时如实写入",
			SortOrder: 20,
		},
		{
			Key: model.SettingAIModels, Value: `[{"id":"qwen3.7-plus","tier":"free","label":"通义千问 3.7 Plus"}]`,
			Kind: model.SettingKindJSON, Group: model.SettingGroupAI,
			Label: "可用模型列表", Hint: `JSON，例如 [{"id":"deepseek-chat","tier":"free","label":"DeepSeek"}]`,
			SortOrder: 30,
		},
		{
			Key: model.SettingAIDefaultModel, Value: "qwen3.7-plus",
			Kind: model.SettingKindText, Group: model.SettingGroupAI,
			Label: "默认免费模型", Hint: "登录用户的免费额度默认使用此模型",
			SortOrder: 40,
		},

		// --- Quotas ---
		{
			Key: model.SettingDailyFreeQuota, Value: "5",
			Kind: model.SettingKindNumber, Group: model.SettingGroupFeature,
			Label: "每日免费 AI 解读次数", Hint: "登录用户每日可享的免费额度",
			SortOrder: 110,
		},
	}
}
