package testsupport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// DispatcherPool uses the shipped grants with a distinct randomly named role.
// It cannot insert or mutate product records and never uses migration credentials.
func DispatcherPool(t *testing.T, db *Database) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	password := hex.EncodeToString(seed[:])
	role := "forge_hd_" + password
	quoted := pgx.Identifier{role}.Sanitize()
	if _, err := db.Admin.Exec(ctx, fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s'", quoted, password)); err != nil {
		t.Fatal(err)
	}
	// Register cleanup before grants/pool creation so partial failures are safe.
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		db.Migration.Exec(ctx, "REVOKE UPDATE(state,delivered_at,attempts,lease_token,lease_until,next_attempt_at,blocked,last_error_code) ON forge.commands FROM "+quoted)
		db.Migration.Exec(ctx, "REVOKE ALL ON forge.principals,forge.workloads,forge.versions,forge.runs,forge.commands FROM "+quoted)
		db.Migration.Exec(ctx, "REVOKE ALL ON SCHEMA forge FROM "+quoted)
		if _, err := db.Admin.Exec(ctx, "DROP ROLE "+quoted); err != nil {
			t.Error("dispatcher fixture role cleanup failed")
		}
	})
	grants, err := os.ReadFile(filepath.Join("..", "..", "deploy", "postgres", "dispatcher-grants.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Migration.Exec(ctx, strings.ReplaceAll(string(grants), "forge_dispatcher", quoted)); err != nil {
		t.Fatal(err)
	}
	cfg := db.Pool.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = password
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return pool
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
	_, err = migration.Exec(ctx, fmt.Sprintf(`GRANT USAGE ON SCHEMA forge TO %[1]s;
 GRANT SELECT,INSERT ON forge.principals,forge.workloads,forge.versions,forge.runs TO %[1]s;
 GRANT SELECT ON forge.commands TO %[1]s; GRANT INSERT(id,run_id,kind) ON forge.commands TO %[1]s;
 GRANT UPDATE(name,description,revision,updated_at) ON forge.workloads TO %[1]s;
 GRANT SELECT,INSERT ON forge.provisioning_operations TO %[1]s;
 GRANT UPDATE(action,desired_generation,observed_generation,status,observed_phase,error_code,error_message,deadline,updated_at) ON forge.provisioning_operations TO %[1]s`, quote(runtime)))
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
