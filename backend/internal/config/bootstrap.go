// Package config loads runtime configuration.
//
// There are two layers of configuration:
//
//  1. Bootstrap config (this file): the absolute minimum needed to
//     start the process — how to reach PostgreSQL and which port to
//     listen on. Loaded from environment / .env file only.
//
//  2. Dynamic config (settings table, see model.Setting and
//     store.SettingStore): AI keys, model lists, feature flags, rate
//     limits — everything that should be editable from the admin
//     panel at runtime without a restart.
//
// Only the DATABASE_URL is a true hard dependency. Everything else
// has a sane default so the server boots even with a minimal env.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Bootstrap holds the startup-only configuration.
type Bootstrap struct {
	DatabaseURL string // PostgreSQL DSN, required
	ServerAddr  string // HTTP listen address, e.g. ":8080"
	LogLevel    string // debug | info | warn | error
}

// Load reads the .env file (if present) and environment variables,
// applies defaults, and validates that DATABASE_URL is set.
//
// godotenv loads .env but NEVER overrides real environment variables —
// which means the production platform (Render/Koyeb/Sevalla) can inject
// DATABASE_URL via its dashboard and it will win over the file.
func Load() (*Bootstrap, error) {
	// .env is optional; in production env vars come from the platform.
	_ = godotenv.Load(".env", ".env.local")

	cfg := &Bootstrap{
		DatabaseURL: getenv("DATABASE_URL", ""),
		ServerAddr:  getenv("SERVER_ADDR", ":8080"),
		LogLevel:    strings.ToLower(getenv("LOG_LEVEL", "info")),
	}

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return nil, fmt.Errorf("DATABASE_URL is required (set it in .env or the environment)")
	}
	return cfg, nil
}

// getenv returns the value of the env var named by key, or fallback
// if it is empty / unset.
func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
