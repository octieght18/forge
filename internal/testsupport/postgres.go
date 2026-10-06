package testsupport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/store"
)

type Database struct {
	Pool      *pgxpool.Pool
	Migration *pgx.Conn
	Repo      *store.Repository
	Admin     *pgx.Conn
}

func NewDatabase(t *testing.T) *Database {
	t.Helper()
	dsn := os.Getenv("FORGE_TEST_ADMIN_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("FORGE_REQUIRE_DB_TESTS") == "1" {
			t.Fatal("test database required")
		}
		t.Skip("set FORGE_TEST_ADMIN_DATABASE_URL for HTTP database integration tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("test admin unavailable")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	seed := hex.EncodeToString(random[:])
	name := "forge_http_" + seed
	owner := "forge_hm_" + seed
	runtime := "forge_hr_" + seed
	password := seed
	quote := func(value string) string { return pgx.Identifier{value}.Sanitize() }
	_, err = admin.Exec(ctx, fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s'; CREATE ROLE %s LOGIN PASSWORD '%s'`, quote(owner), password, quote(runtime), password))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec(ctx, "DROP DATABASE IF EXISTS "+quote(name)+" WITH (FORCE)")
		admin.Exec(ctx, "DROP ROLE "+quote(runtime))
		admin.Exec(ctx, "DROP ROLE "+quote(owner))
		admin.Close(ctx)
	})
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quote(name)+" OWNER "+quote(owner)); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("test configuration invalid")
	}
	cfg.Database = name
	cfg.User = owner
	cfg.Password = password
	migration, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal("migration connection failed")
	}
	t.Cleanup(func() { migration.Close(ctx) })
	if err := store.Migrate(ctx, migration); err != nil {
		t.Fatal(err)
	}
	_, err = migration.Exec(ctx, fmt.Sprintf(`GRANT USAGE ON SCHEMA forge TO %s; GRANT SELECT,INSERT ON forge.principals,forge.workloads,forge.versions,forge.runs,forge.commands TO %s; GRANT UPDATE(name,description,revision,updated_at) ON forge.workloads TO %s`, quote(runtime), quote(runtime), quote(runtime)))
	if err != nil {
		t.Fatal(err)
	}
	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("pool configuration invalid")
	}
	pcfg.ConnConfig.Database = name
	pcfg.ConnConfig.User = runtime
	pcfg.ConnConfig.Password = password
	pcfg.MaxConns = 12
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repo, err := store.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	return &Database{pool, migration, repo, admin}
}
