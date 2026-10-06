package store

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies forward-only SQL under one transaction-level advisory lock.
// It requires a dedicated migration connection, not the runtime role.
func Migrate(ctx context.Context, conn *pgx.Conn) error {
	return migrateFS(ctx, conn, migrations)
}

func migrateFS(ctx context.Context, conn *pgx.Conn, files fs.FS) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7091134835021)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS forge;
        REVOKE ALL ON SCHEMA forge FROM PUBLIC;
        CREATE TABLE IF NOT EXISTS forge.schema_migrations (
            name text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
        ); REVOKE ALL ON forge.schema_migrations FROM PUBLIC;`); err != nil {
		return err
	}
	names, err := fs.Glob(files, "migrations/*.sql")
	if err != nil || len(names) == 0 {
		return errors.New("missing migrations")
	}
	sort.Strings(names)
	expected := make(map[string]string, len(names))
	bodies := make(map[string][]byte, len(names))
	for _, name := range names {
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		expected[name] = hex.EncodeToString(sum[:])
		bodies[name] = body
	}
	rows, err := tx.Query(ctx, `SELECT name, checksum FROM forge.schema_migrations ORDER BY name`)
	if err != nil {
		return err
	}
	applied := make(map[string]bool)
	for rows.Next() {
		var name, checksum string
		if err := rows.Scan(&name, &checksum); err != nil {
			rows.Close()
			return err
		}
		if expected[name] != checksum {
			rows.Close()
			return errors.New("migration history or checksum mismatch")
		}
		applied[name] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	// An applied later migration with a missing earlier one is corrupt history.
	missing := false
	for _, name := range names {
		if !applied[name] {
			missing = true
			continue
		}
		if missing {
			return errors.New("migration history has a gap")
		}
	}
	for _, name := range names {
		if applied[name] {
			continue
		}
		if _, err := tx.Exec(ctx, string(bodies[name])); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO forge.schema_migrations(name, checksum) VALUES ($1,$2)`, name, expected[name]); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
