package sqlite

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	// modernc.org/sqlite provides the pure-Go sqlite driver.
	_ "modernc.org/sqlite"
)

// ErrNotFound is returned by repository Get/Delete when the requested
// record doesn't exist. The JSON-file adapter returns the same sentinel.
var ErrNotFound = errors.New("not found")

// fmtErr wraps an error with additional context.
func fmtErr(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens (creating if needed) the SQLite database at dbPath, applies
// any pending migrations, and returns the *sql.DB. The caller owns closing
// it.
func Open(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create sqlite dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// migrate applies each migrations/*.sql file whose numeric prefix exceeds
// the current schema_migrations max version, in filename order, each inside
// its own transaction. Minimal hand-rolled runner — no external migration
// library.
func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL
	)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	current, err := currentVersion(db)
	if err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	var pending []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		ver, err := migrationVersion(name)
		if err != nil {
			return err
		}
		if ver > current {
			pending = append(pending, name)
		}
	}
	sort.Strings(pending)

	for _, name := range pending {
		ver, _ := migrationVersion(name)
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`,
			ver, time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// currentVersion returns the highest applied migration version, or 0 if
// none are recorded.
func currentVersion(db *sql.DB) (int, error) {
	var maxVersion sql.NullInt64
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&maxVersion); err != nil {
		return 0, fmt.Errorf("read schema migrations: %w", err)
	}
	return int(maxVersion.Int64), nil
}

// migrationVersion extracts the leading numeric prefix of a migration
// filename, e.g. "0001_init.sql" -> 1.
func migrationVersion(name string) (int, error) {
	base := strings.TrimSuffix(name, ".sql")
	idx := strings.IndexAny(base, "_")
	if idx < 0 {
		idx = len(base)
	}
	prefix := base[:idx]
	v, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("invalid migration filename %q: expected numeric prefix", name)
	}
	return v, nil
}
