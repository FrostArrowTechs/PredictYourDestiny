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
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

// Bootstrap holds the startup-only configuration.
type Bootstrap struct {
	DatabaseURL              string          // PostgreSQL DSN, required
	DatabaseConfig           *pgx.ConnConfig // parsed connection config used by PostgreSQL clients
	ServerAddr               string          // HTTP listen address, e.g. ":8080"
	JWTSecret                string          // JWT signing secret, required for auth
	AIProviderEncryptionKey  string          // base64-encoded 32-byte AES key
	Environment              string          // development | production
	CORSAllowedOrigins       []string        // exact browser origins allowed by the API
	LogLevel                 string          // debug | info | warn | error
	HistoryRetentionDays     int             // generated records/chats; 0 disables automatic purge
	ReservationRetentionDays int             // AI idempotency rows; 0 disables automatic purge
}

// Load reads the .env file (if present) and environment variables,
// applies defaults, and validates that DATABASE_URL is set.
//
// If the target database does not exist but the connection has
// permission to create it, the database is created on the fly so
// first-time users don't have to run psql by hand.
//
// godotenv loads .env but NEVER overrides real environment variables —
// which means the production platform (Render/Koyeb/Sevella) can inject
// DATABASE_URL via its dashboard and it will win over the file.
func Load() (*Bootstrap, error) {
	// .env is optional; in production env vars come from the platform.
	_ = godotenv.Load(".env", ".env.local")

	cfg := &Bootstrap{
		DatabaseURL:              getenv("DATABASE_URL", ""),
		ServerAddr:               getenv("SERVER_ADDR", ":8080"),
		JWTSecret:                getenv("JWT_SECRET", ""),
		AIProviderEncryptionKey:  getenv("AI_PROVIDER_ENCRYPTION_KEY", ""),
		Environment:              strings.ToLower(getenv("APP_ENV", "development")),
		CORSAllowedOrigins:       splitCSV(getenv("CORS_ALLOWED_ORIGINS", "")),
		LogLevel:                 strings.ToLower(getenv("LOG_LEVEL", "info")),
		HistoryRetentionDays:     getenvInt("HISTORY_RETENTION_DAYS", 365),
		ReservationRetentionDays: getenvInt("AI_RESERVATION_RETENTION_DAYS", 30),
	}

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return nil, fmt.Errorf("DATABASE_URL is required (set it in .env or the environment)")
	}
	if cfg.Environment == "production" && len(cfg.CORSAllowedOrigins) == 0 {
		return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS is required when APP_ENV=production")
	}

	// Normalize the dbname. If the DSN points at the default `postgres`
	// system database (common on fresh managed PG instances) we rewrite
	// it to `predictdestiny` so the application's tables don't pollute
	// the system catalog. Operators can still target any other database
	// by setting dbname= explicitly.
	databaseConfig, err := normalizeDBName(cfg.DatabaseURL, "predictdestiny")
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.DatabaseConfig = databaseConfig

	// Make sure the target database exists. If not, we try to create
	// it. This means a fresh checkout can boot against an empty PG
	// instance with no manual psql steps.
	if err := ensureDatabase(cfg.DatabaseConfig); err != nil {
		return nil, fmt.Errorf("ensure database: %w", err)
	}

	return cfg, nil
}

// normalizeDBName rewrites the dbname in a libpq DSN. If the DSN
// has no dbname at all, or targets the system `postgres` database,
// the target is changed to the supplied default (e.g. "predictdestiny").
// Any other explicit dbname is left alone so operators can still point
// at their own dedicated database.
func normalizeDBName(dsn, defaultDB string) (*pgx.ConnConfig, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if !hasExplicitDatabase(dsn) || config.Database == "" || config.Database == "postgres" {
		config.Database = defaultDB
	}
	return config, nil
}

var keywordDatabase = regexp.MustCompile(`(?i)(^|\s)dbname\s*=`)

func hasExplicitDatabase(dsn string) bool {
	trimmed := strings.TrimSpace(dsn)
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		u, err := url.Parse(trimmed)
		return err == nil && u.Path != "" && u.Path != "/"
	}
	return keywordDatabase.MatchString(trimmed)
}

// ensureDatabase guarantees that the database named in the DSN
// already exists; if it does not, it tries to create it.
//
// Behavior:
//   - If the database exists: returns nil immediately.
//   - If the database does not exist AND the user has CREATEDB
//     privilege: creates it and returns nil.
//   - Otherwise: returns a descriptive error so the operator can
//     either create it manually or set DATABASE_URL to an existing
//     database they own.
//
// The check is idempotent and safe to call on every boot.
func ensureDatabase(config *pgx.ConnConfig) error {
	targetDB := config.Database
	adminConfig := config.Copy()
	adminConfig.Database = "postgres"
	if targetDB == "" {
		return fmt.Errorf("DATABASE_URL is missing a database name")
	}

	// Open a connection to the admin database (postgres) to issue
	// the CREATE DATABASE if needed. We always connect to "postgres"
	// first because the target database is, by definition, not yet
	// connectable if it doesn't exist.
	admin := stdlib.OpenDB(*adminConfig)
	defer admin.Close()

	if err := admin.Ping(); err != nil {
		// Surface a clearer hint when the host/credentials are wrong.
		return fmt.Errorf("ping admin db (check host/user/password): %w", err)
	}

	// Check existence. We use to_regclass to avoid driver quirks.
	var exists bool
	row := admin.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`,
		targetDB,
	)
	if err := row.Scan(&exists); err != nil {
		return fmt.Errorf("check database existence: %w", err)
	}
	if exists {
		return nil
	}

	// pgx.Identifier quotes the name as a PostgreSQL identifier, preventing
	// injection while still permitting legitimate names such as "app-prod".
	if _, err := admin.Exec("CREATE DATABASE " + pgx.Identifier{targetDB}.Sanitize()); err != nil {
		// Most likely cause: the user lacks CREATEDB privilege. Give
		// a clear, actionable error so the operator doesn't have to
		// guess what to do next.
		return fmt.Errorf(
			"database %q does not exist and could not be created automatically (%v).\n"+
				"Either run `CREATE DATABASE %s;` as a superuser, or set DATABASE_URL\n"+
				"to a database that already exists and that the configured user owns",
			targetDB, err, targetDB,
		)
	}

	fmt.Printf("✓ Created database %q\n", targetDB)
	return nil
}

// splitDSN parses either a URL or keyword DSN and returns
//
//	(targetDB, adminConfig) where adminConfig is the same connection rewritten
//	to point at the `postgres` database (used for the existence check
//	and the CREATE DATABASE statement).
func splitDSN(dsn string) (string, *pgx.ConnConfig, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return "", nil, fmt.Errorf("parse database config: %w", err)
	}
	target := config.Database
	admin := config.Copy()
	admin.Database = "postgres"
	return target, admin, nil
}

// getenv returns the value of the env var named by key, or fallback
// if it is empty / unset.
func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := getenv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, strings.TrimRight(item, "/"))
		}
	}
	return values
}
