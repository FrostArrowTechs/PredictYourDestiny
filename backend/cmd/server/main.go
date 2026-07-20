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
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/6tail/lunar-go/calendar" // sanity-check the dep is resolvable
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/auth"
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

	// 1.5) initialize JWT auth
	if cfg.JWTSecret == "" {
		log.Printf("warn: JWT_SECRET not set, auth endpoints will not work")
	} else {
		auth.InitJWT(cfg.JWTSecret)
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
		&model.DreamEntry{},
		&model.DivinationPoem{},
		&model.CharacterStroke{},
		&model.TarotCard{},
		&model.AIProvider{},
		&model.MembershipTier{},
		&model.UserMembership{},
	); err != nil {
		log.Fatalf("automigrate: %v", err)
	}
	log.Println("database ready, schema migrated")

	// 3.5) seed dream entries (idempotent)
	if err := seedDreamEntries(db); err != nil {
		log.Printf("warn: seed dream entries: %v", err)
	}

	// 3.6) seed divination poems (idempotent)
	if err := seedDivinationPoems(db); err != nil {
		log.Printf("warn: seed divination poems: %v", err)
	}

	// 3.7) seed character strokes (idempotent)
	if err := seedCharacterStrokes(db); err != nil {
		log.Printf("warn: seed character strokes: %v", err)
	}

	// 3.8) seed tarot cards (idempotent)
	if err := seedTarotCards(db); err != nil {
		log.Printf("warn: seed tarot cards: %v", err)
	}

	// 3.9) seed membership tiers (idempotent)
	if err := seedMembershipTiers(db); err != nil {
		log.Printf("warn: seed membership tiers: %v", err)
	}

	// 3.10) bootstrap the first admin when explicit credentials are provided
	if err := seedAdminUser(db); err != nil {
		log.Printf("warn: seed admin user: %v", err)
	}

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

// seedDreamEntries loads seed/dream.json into the dream_entries table.
// It's idempotent: rows already present are skipped (by keyword).
func seedDreamEntries(db *gorm.DB) error {
	// Read seed file
	data, err := os.ReadFile("seed/dream.json")
	if err != nil {
		return err
	}

	var entries []model.DreamEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	// Insert only if keyword doesn't exist
	var count int64
	db.Model(&model.DreamEntry{}).Count(&count)
	if count > 0 {
		log.Printf("dream_entries already seeded (%d rows), skipping", count)
		return nil
	}

	if err := db.Create(&entries).Error; err != nil {
		return err
	}
	log.Printf("seeded %d dream entries", len(entries))
	return nil
}

// seedDivinationPoems loads seed/divination.json into the
// divination_poems table. Idempotent: skipped if rows already exist.
func seedDivinationPoems(db *gorm.DB) error {
	data, err := os.ReadFile("seed/divination.json")
	if err != nil {
		return err
	}

	var poems []model.DivinationPoem
	if err := json.Unmarshal(data, &poems); err != nil {
		return err
	}

	var count int64
	db.Model(&model.DivinationPoem{}).Count(&count)
	if count > 0 {
		log.Printf("divination_poems already seeded (%d rows), skipping", count)
		return nil
	}

	if err := db.Create(&poems).Error; err != nil {
		return err
	}
	log.Printf("seeded %d divination poems", len(poems))
	return nil
}

// seedCharacterStrokes loads seed/strokes.json into the character_strokes
// table. Idempotent: skipped if rows already exist.
func seedCharacterStrokes(db *gorm.DB) error {
	data, err := os.ReadFile("seed/strokes.json")
	if err != nil {
		return err
	}

	var strokes []model.CharacterStroke
	if err := json.Unmarshal(data, &strokes); err != nil {
		return err
	}

	var count int64
	db.Model(&model.CharacterStroke{}).Count(&count)
	if count > 0 {
		log.Printf("character_strokes already seeded (%d rows), skipping", count)
		return nil
	}

	if err := db.Create(&strokes).Error; err != nil {
		return err
	}
	log.Printf("seeded %d character strokes", len(strokes))
	return nil
}

// seedTarotCards loads seed/tarot.json (78-card Rider-Waite deck) into
// the tarot_cards table. Idempotent: skipped if rows already exist.
func seedTarotCards(db *gorm.DB) error {
	data, err := os.ReadFile("seed/tarot.json")
	if err != nil {
		return err
	}

	var cards []model.TarotCard
	if err := json.Unmarshal(data, &cards); err != nil {
		return err
	}

	var count int64
	db.Model(&model.TarotCard{}).Count(&count)
	if count > 0 {
		log.Printf("tarot_cards already seeded (%d rows), skipping", count)
		return nil
	}

	if err := db.Create(&cards).Error; err != nil {
		return err
	}
	log.Printf("seeded %d tarot cards", len(cards))
	return nil
}

// seedMembershipTiers creates the default membership tiers (free/basic/premium)
// if they don't already exist.
func seedMembershipTiers(db *gorm.DB) error {
	tiers := []model.MembershipTier{
		{
			Code:       model.TierCodeFree,
			Name:       "免费用户",
			DailyQuota: 5,
			Features:   `["basic_interpret"]`,
			PriceMonth: 0,
			SortOrder:  1,
		},
		{
			Code:       model.TierCodeBasic,
			Name:       "基础会员",
			DailyQuota: 20,
			Features:   `["basic_interpret","deep_interpret","history_save"]`,
			PriceMonth: 990, // ¥9.90/month
			SortOrder:  2,
		},
		{
			Code:       model.TierCodePremium,
			Name:       "高级会员",
			DailyQuota: -1, // unlimited
			Features:   `["basic_interpret","deep_interpret","history_save","priority_support"]`,
			PriceMonth: 2990, // ¥29.90/month
			SortOrder:  3,
		},
	}

	for _, tier := range tiers {
		var existing model.MembershipTier
		if err := db.Where("code = ?", tier.Code).First(&existing).Error; err == gorm.ErrRecordNotFound {
			if err := db.Create(&tier).Error; err != nil {
				return err
			}
			log.Printf("seeded membership tier: %s", tier.Code)
		}
	}
	return nil
}

// seedAdminUser creates the first admin only when the operator provides
// explicit bootstrap credentials.
//
// Credentials are read from env (ADMIN_EMAIL / ADMIN_PASSWORD) and
// are never defaulted or written to logs.
//
// Idempotent: if any admin already exists, this is a no-op so
// password changes via the admin panel are preserved across restarts.
func seedAdminUser(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	email := strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))
	password := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	if email == "" && password == "" {
		log.Printf("admin bootstrap skipped: set ADMIN_EMAIL and ADMIN_PASSWORD to create the first admin")
		return nil
	}
	if email == "" || len(password) < 12 {
		return errors.New("ADMIN_EMAIL and an ADMIN_PASSWORD of at least 12 characters are required")
	}

	hashed, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	admin := model.User{
		Email:       email,
		Password:    hashed,
		DisplayName: "Administrator",
		Role:        "admin",
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	log.Printf("bootstrap admin account created for %s", email)
	return nil
}
