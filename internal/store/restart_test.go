package store

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The harness invokes seed, restarts the actual PostgreSQL service, then invokes
// verify in another Go process. The proof contains IDs only, never credentials.
func TestPostgreSQLRestartPersistence(t *testing.T) {
	phase := os.Getenv("FORGE_TEST_RESTART_PHASE")
	if phase == "" {
		t.Skip("restart harness invokes seed and verify separately")
	}
	if phase != "seed" && phase != "verify" {
		t.Fatal("unknown restart phase")
	}
	path := os.Getenv("FORGE_RESTART_PROOF_FILE")
	if path == "" {
		t.Fatal("FORGE_RESTART_PROOF_FILE required")
	}
	ctx := context.Background()
	dsn := os.Getenv("FORGE_TEST_ADMIN_DATABASE_URL")
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("restart admin connection failed")
	}
	defer admin.Close(ctx)
	var proof struct{ Database, WorkloadID, VersionID, RunID, WorkflowID string }
	if phase == "seed" {
		id, err := uuid()
		if err != nil {
			t.Fatal(err)
		}
		proof.Database = "forge_restart_" + strings.ReplaceAll(id, "-", "")
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{proof.Database}.Sanitize()); err != nil {
			t.Fatal(err)
		}
	} else {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &proof); err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^forge_restart_[0-9a-f]{32}$`).MatchString(proof.Database) {
			t.Fatal("invalid disposable database identity")
		}
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("restart DSN invalid")
	}
	cfg.ConnConfig.Database = proof.Database
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if phase == "seed" {
		c, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		err = Migrate(ctx, c.Conn())
		c.Release()
		if err != nil {
			t.Fatal(err)
		}
	}
	r, err := New(pool)
	if err != nil {
		t.Fatal(err)
	}
	if phase == "seed" {
		w := createWorkload(t, r, devA, "restart-proof")
		v := createVersion(t, r, devA, w)
		run, err := r.SubmitRun(ctx, devA, "restart-proof", runPayload(w, v))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.Cancel(ctx, devA, run.ID); err != nil {
			t.Fatal(err)
		}
		proof.WorkloadID = w.ID
		proof.VersionID = v.ID
		proof.RunID = run.ID
		proof.WorkflowID = run.WorkflowID
		b, _ := json.Marshal(proof)
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("restart proof seeded; database must now be restarted by harness")
	} else {
		w, err := r.GetWorkload(ctx, devA, proof.WorkloadID)
		if err != nil {
			t.Fatal(err)
		}
		v, err := r.GetVersion(ctx, devA, w.ID, proof.VersionID)
		if err != nil {
			t.Fatal(err)
		}
		run, err := r.SubmitRun(ctx, devA, "restart-proof", runPayload(w, v))
		if err != nil || run.ID != proof.RunID || run.WorkflowID != proof.WorkflowID {
			t.Fatal("restart lost acceptance identity", err)
		}
		commands, err := r.Commands(ctx, devA, run.ID)
		if err != nil || len(commands) != 2 || commands[0].State != "pending" || commands[1].State != "pending" {
			t.Fatal("restart lost commands", err)
		}
		pool.Close()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{proof.Database}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Fatal(err)
		}
		t.Log("database restart retained workload, version, run, key and both commands")
	}
}
