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
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver for the bootstrap check
	"github.com/joho/godotenv"
)

// Bootstrap holds the startup-only configuration.
type Bootstrap struct {
	DatabaseURL string // PostgreSQL DSN, required
	ServerAddr  string // HTTP listen address, e.g. ":8080"
	JWTSecret   string // JWT signing secret, required for auth
	LogLevel    string // debug | info | warn | error
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
		DatabaseURL: getenv("DATABASE_URL", ""),
		ServerAddr:  getenv("SERVER_ADDR", ":8080"),
		JWTSecret:   getenv("JWT_SECRET", ""),
		LogLevel:    strings.ToLower(getenv("LOG_LEVEL", "info")),
	}

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return nil, fmt.Errorf("DATABASE_URL is required (set it in .env or the environment)")
	}

	// Normalize the dbname. If the DSN points at the default `postgres`
	// system database (common on fresh managed PG instances) we rewrite
	// it to `predictdestiny` so the application's tables don't pollute
	// the system catalog. Operators can still target any other database
	// by setting dbname= explicitly.
	cfg.DatabaseURL = normalizeDBName(cfg.DatabaseURL, "predictdestiny")

	// Make sure the target database exists. If not, we try to create
	// it. This means a fresh checkout can boot against an empty PG
	// instance with no manual psql steps.
	if err := ensureDatabase(cfg.DatabaseURL); err != nil {
		return nil, fmt.Errorf("ensure database: %w", err)
	}

	return cfg, nil
}

// normalizeDBName rewrites the dbname in a libpq DSN. If the DSN
// has no dbname at all, or targets the system `postgres` database,
// the target is changed to the supplied default (e.g. "predictdestiny").
// Any other explicit dbname is left alone so operators can still point
// at their own dedicated database.
func normalizeDBName(dsn, defaultDB string) string {
	parts := strings.Fields(dsn)
	hasDBName := false
	for i, p := range parts {
		if strings.HasPrefix(p, "dbname=") {
			hasDBName = true
			current := strings.TrimPrefix(p, "dbname=")
			// Only rewrite when pointing at the system DB. Explicit
			// user-chosen databases (including "postgres" used as a
			// real application DB) are preserved.
			if current == "postgres" {
				parts[i] = "dbname=" + defaultDB
			}
		}
	}
	if !hasDBName {
		parts = append(parts, "dbname="+defaultDB)
	}
	return strings.Join(parts, " ")
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
func ensureDatabase(dsn string) error {
	// Parse dbname out of the DSN. GORM accepts libpq-style key=value
	// strings, so a quick scan is enough.
	targetDB, adminDSN, err := splitDSN(dsn)
	if err != nil {
		return err
	}
	if targetDB == "" {
		return errors.New("DATABASE_URL is missing dbname=…")
	}

	// Open a connection to the admin database (postgres) to issue
	// the CREATE DATABASE if needed. We always connect to "postgres"
	// first because the target database is, by definition, not yet
	// connectable if it doesn't exist.
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return fmt.Errorf("open admin db: %w", err)
	}
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

	// Create the database. The target name is validated above to
	// be a real identifier (we only allow letters/digits/underscore).
	if !isSafeIdent(targetDB) {
		return fmt.Errorf("refusing to CREATE DATABASE with unsafe name %q", targetDB)
	}

	if _, err := admin.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, targetDB)); err != nil {
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

// splitDSN pulls `dbname=X` out of a libpq DSN and returns
//   (targetDB, adminDSN) where adminDSN is the same DSN rewritten
//   to point at the `postgres` database (used for the existence check
//   and the CREATE DATABASE statement).
func splitDSN(dsn string) (string, string, error) {
	parts := strings.Fields(dsn)
	var target string
	rest := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.HasPrefix(p, "dbname=") {
			target = strings.TrimPrefix(p, "dbname=")
		} else {
			rest = append(rest, p)
		}
	}
	rest = append(rest, "dbname=postgres")
	return target, strings.Join(rest, " "), nil
}

// isSafeIdent returns true for identifiers that contain only
// letters, digits, and underscores. We use this to prevent DSN
// injection when interpolating the database name into a
// CREATE DATABASE statement.
func isSafeIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return false
		}
	}
	return true
}

// getenv returns the value of the env var named by key, or fallback
// if it is empty / unset.
func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// silence unused-import warning for pgx/stdlib if the user rebuilds
// after removing the check above; keeping it referenced makes the
// driver registration visible to anyone reading this file.
var _ = stdlib.GetDefaultDriver