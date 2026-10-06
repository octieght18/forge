package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/contract"
)

type database struct {
	admin, migrator *pgx.Conn
	pool            *pgxpool.Pool
	repo            *Repository
	runtimeConfig   *pgxpool.Config
	migratorConfig  *pgx.ConnConfig
}

var devA = Principal{"https://identity.example/realms/forge", "a", "developer"}
var devB = Principal{"https://identity.example/realms/forge", "b", "developer"}
var operator = Principal{"https://identity.example/realms/forge", "operator", "operator"}

// Each test owns a new database and two roles. It never drops a supplied database.
func testDatabase(t *testing.T) *database {
	t.Helper()
	dsn := os.Getenv("FORGE_TEST_ADMIN_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("FORGE_REQUIRE_DB_TESTS") == "1" {
			t.Fatal("FORGE_TEST_ADMIN_DATABASE_URL required")
		}
		t.Skip("set FORGE_TEST_ADMIN_DATABASE_URL for real PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("test admin connection failed")
	}
	seed, err := uuid()
	if err != nil {
		t.Fatal(err)
	}
	seed = strings.ReplaceAll(seed, "-", "")
	dbname := "forge_test_" + seed
	migrator := "forge_m_" + seed
	runtime := "forge_r_" + seed
	password, err := uuid()
	if err != nil {
		t.Fatal(err)
	}
	quote := func(name string) string { return pgx.Identifier{name}.Sanitize() }
	_, err = admin.Exec(ctx, fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s'; CREATE ROLE %s LOGIN PASSWORD '%s'`, quote(migrator), password, quote(runtime), password))
	if err != nil {
		admin.Close(ctx)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec(ctx, "DROP DATABASE IF EXISTS "+quote(dbname)+" WITH (FORCE)")
		admin.Exec(ctx, "DROP ROLE "+quote(runtime))
		admin.Exec(ctx, "DROP ROLE "+quote(migrator))
		admin.Close(ctx)
	})
	_, err = admin.Exec(ctx, "CREATE DATABASE "+quote(dbname)+" OWNER "+quote(migrator))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	cfg.Database = dbname
	cfg.User = migrator
	cfg.Password = password
	migrationConn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal("migration connection failed")
	}
	t.Cleanup(func() { migrationConn.Close(ctx) })
	if err := Migrate(ctx, migrationConn); err != nil {
		t.Fatal(err)
	}
	// These match the documented operator grants; no migration ownership for runtime.
	_, err = migrationConn.Exec(ctx, fmt.Sprintf(`GRANT USAGE ON SCHEMA forge TO %s;
        GRANT SELECT,INSERT ON forge.principals,forge.workloads,forge.versions,forge.runs,forge.commands TO %s;
        GRANT UPDATE(name,description,revision,updated_at) ON forge.workloads TO %s;`, quote(runtime), quote(runtime), quote(runtime)))
	if err != nil {
		t.Fatal(err)
	}
	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid pool DSN")
	}
	pcfg.ConnConfig.Database = dbname
	pcfg.ConnConfig.User = runtime
	pcfg.ConnConfig.Password = password
	pcfg.MaxConns = 12
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal("runtime connection failed")
	}
	repo, err := New(pool)
	if err != nil {
		t.Fatal(err)
	}
	return &database{admin, migrationConn, pool, repo, pcfg, cfg}
}
func createWorkload(t *testing.T, r *Repository, p Principal, name string) Workload {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"name": name, "description": "fixture"})
	w, err := r.CreateWorkload(context.Background(), p, payload)
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func createVersion(t *testing.T, r *Repository, p Principal, w Workload) Version {
	t.Helper()
	payload, err := contract.Examples.ReadFile("examples/create-version.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := r.CreateVersion(context.Background(), p, w.ID, payload)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func runPayload(w Workload, v Version) []byte {
	b, _ := json.Marshal(runInput{w.ID, v.ID, "What owns product state?", []string{"platform-brief"}})
	return b
}
func requireError(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}
func count(t *testing.T, conn *pgx.Conn, table string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(context.Background(), "SELECT count(*) FROM forge."+pgx.Identifier{table}.Sanitize()).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPostgreSQLRepositories(t *testing.T) {
	db := testDatabase(t)
	r := db.repo
	ctx := context.Background()
	t.Run("workload ownership and optimistic update", func(t *testing.T) {
		w := createWorkload(t, r, devA, "owner-check")
		_, err := r.GetWorkload(ctx, devB, w.ID)
		requireError(t, err, ErrNotFound)
		got, err := r.GetWorkload(ctx, operator, w.ID)
		if err != nil || got.Owner.Subject != "a" {
			t.Fatalf("operator inspection: %v", err)
		}
		_, err = r.UpdateWorkload(ctx, operator, w.ID, 1, []byte(`{"description":"operator write"}`))
		requireError(t, err, ErrNotFound)
		_, err = r.UpdateWorkload(ctx, devB, w.ID, 999, []byte(`{"description":"foreign"}`))
		requireError(t, err, ErrNotFound)
		got, err = r.UpdateWorkload(ctx, devA, w.ID, 1, []byte(`{"description":"updated"}`))
		if err != nil || got.Revision != 2 || got.Description != "updated" {
			t.Fatalf("update: %+v %v", got, err)
		}
		_, err = r.UpdateWorkload(ctx, devA, w.ID, 1, []byte(`{"name":"stale"}`))
		requireError(t, err, ErrPrecondition)
		createWorkload(t, r, devB, "owner-check")
		_, err = r.CreateWorkload(ctx, devA, []byte(`{"name":"owner-check","description":"duplicate"}`))
		requireError(t, err, ErrConflict)
		b, _ := json.Marshal(got)
		if err := r.validator.Validate("Workload", b); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("concurrent name creation and revision", func(t *testing.T) {
		const n = 8
		var wg sync.WaitGroup
		results := make(chan error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := r.CreateWorkload(ctx, devA, []byte(`{"name":"race-create","description":"race"}`))
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		success := 0
		for err := range results {
			if err == nil {
				success++
			} else {
				requireError(t, err, ErrConflict)
			}
		}
		if success != 1 {
			t.Fatalf("created %d workloads", success)
		}
		w := createWorkload(t, r, devA, "race-update")
		results = make(chan error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := r.UpdateWorkload(ctx, devA, w.ID, 1, []byte(`{"description":"one winner"}`))
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		success = 0
		for err := range results {
			if err == nil {
				success++
			} else {
				requireError(t, err, ErrPrecondition)
			}
		}
		if success != 1 {
			t.Fatalf("updated %d times", success)
		}
	})
	t.Run("malformed input leaves no records", func(t *testing.T) {
		before := count(t, db.migrator, "workloads")
		for _, payload := range []string{`{"name":"valid","owner":"b"}`, `{"name":"valid","name":"duplicate"}`, `{"name":"../../etc"}`, `{"name":null}`, `{"name":"valid","description":"\u0000"}`} {
			if _, err := r.CreateWorkload(ctx, devA, []byte(payload)); err == nil {
				t.Fatalf("accepted %s", payload)
			}
		}
		if count(t, db.migrator, "workloads") != before {
			t.Fatal("malformed input persisted")
		}
		_, err := r.CreateWorkload(ctx, Principal{devA.Issuer, "a", "admin"}, []byte(`{"name":"role"}`))
		requireError(t, err, ErrForbidden)
		_, err = r.GetWorkload(ctx, devA, "not-a-uuid")
		requireError(t, err, contract.ErrInvalidInput)
	})
	t.Run("versions are serialized and immutable", func(t *testing.T) {
		w := createWorkload(t, r, devA, "version-race")
		payload, _ := contract.Examples.ReadFile("examples/create-version.json")
		const n = 8
		results := make(chan Version, n)
		failures := make(chan error, n)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				v, err := r.CreateVersion(ctx, devA, w.ID, payload)
				results <- v
				failures <- err
			}()
		}
		wg.Wait()
		close(results)
		close(failures)
		for err := range failures {
			if err != nil {
				t.Fatal(err)
			}
		}
		numbers := map[int64]bool{}
		var first Version
		for v := range results {
			if numbers[v.Number] {
				t.Fatal("duplicate version number")
			}
			numbers[v.Number] = true
			first = v
		}
		encoded, _ := json.Marshal(first)
		if err := r.validator.Validate("WorkloadVersion", encoded); err != nil {
			t.Fatal(err)
		}
		if len(numbers) != n || !numbers[1] || !numbers[n] {
			t.Fatal(numbers)
		}
		_, err := r.CreateVersion(ctx, operator, w.ID, payload)
		requireError(t, err, ErrNotFound)
		_, err = r.GetVersion(ctx, devB, w.ID, first.ID)
		requireError(t, err, ErrNotFound)
		w2 := createWorkload(t, r, devA, "version-other")
		_, err = r.GetVersion(ctx, devA, w2.ID, first.ID)
		requireError(t, err, ErrNotFound)
		_, err = db.migrator.Exec(ctx, `UPDATE forge.versions SET spec='{}' WHERE id=$1`, first.ID)
		if err == nil {
			t.Fatal("immutable version updated")
		}
		_, err = db.migrator.Exec(ctx, `DELETE FROM forge.versions WHERE id=$1`, first.ID)
		if err == nil {
			t.Fatal("immutable version deleted")
		}
		page, err := r.ListVersions(ctx, devA, w.ID, 3, 0)
		if err != nil || len(page) != 3 || page[0].Number != 8 {
			t.Fatalf("page: %v %v", page, err)
		}
		next, err := r.ListVersions(ctx, devA, w.ID, 3, page[2].Number)
		if err != nil || len(next) != 3 || next[0].Number != 5 {
			t.Fatalf("next: %v %v", next, err)
		}
	})
	t.Run("atomic acceptance and idempotency races", func(t *testing.T) {
		w := createWorkload(t, r, devA, "run-race")
		v := createVersion(t, r, devA, w)
		payload := runPayload(w, v)
		const n = 8
		results := make(chan Run, n)
		failures := make(chan error, n)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				run, err := r.SubmitRun(ctx, devA, "race-key", payload)
				results <- run
				failures <- err
			}()
		}
		wg.Wait()
		close(results)
		close(failures)
		for err := range failures {
			if err != nil {
				t.Fatal(err)
			}
		}
		ids := map[string]bool{}
		var run Run
		for result := range results {
			ids[result.ID] = true
			run = result
		}
		if len(ids) != 1 {
			t.Fatal("duplicate runs", ids)
		}
		commands, err := r.Commands(ctx, devA, run.ID)
		if err != nil || len(commands) != 1 || commands[0].Kind != "start" || commands[0].State != "pending" {
			t.Fatalf("commands: %v %v", commands, err)
		}
		if run.WorkflowID != "forge-run/"+run.ID {
			t.Fatal("unstable workflow identity")
		}
		changed := strings.Replace(string(payload), "What owns product state?", "Different question?", 1)
		_, err = r.SubmitRun(ctx, devA, "race-key", []byte(changed))
		requireError(t, err, ErrConflict)
		_, err = r.SubmitRun(ctx, devB, "race-key", payload)
		requireError(t, err, ErrNotFound)
		_, err = r.SubmitRun(ctx, operator, "operator-key", payload)
		requireError(t, err, ErrNotFound)
		_, err = r.GetRun(ctx, devB, run.ID)
		requireError(t, err, ErrNotFound)
		if _, err := r.GetRun(ctx, operator, run.ID); err != nil {
			t.Fatal(err)
		}
		_, err = r.SubmitRun(ctx, devA, "bad-key", []byte(strings.Replace(string(payload), "platform-brief", "unapproved-document", 1)))
		requireError(t, err, contract.ErrInvalidInput)
		_, err = r.SubmitRun(ctx, devA, "spaces not allowed", payload)
		requireError(t, err, contract.ErrInvalidInput)
		// Cancellation is one durable command even during pending start.
		cancelIDs := make(chan string, n)
		failures = make(chan error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); c, err := r.Cancel(ctx, devA, run.ID); cancelIDs <- c.ID; failures <- err }()
		}
		wg.Wait()
		close(cancelIDs)
		close(failures)
		unique := map[string]bool{}
		for err := range failures {
			if err != nil {
				t.Fatal(err)
			}
		}
		for id := range cancelIDs {
			unique[id] = true
		}
		if len(unique) != 1 {
			t.Fatal("duplicate cancellation")
		}
		_, err = r.Cancel(ctx, operator, run.ID)
		requireError(t, err, ErrNotFound)
		commands, err = r.Commands(ctx, operator, run.ID)
		if err != nil || len(commands) != 2 || commands[0].Kind != "start" || commands[1].Kind != "cancel" {
			t.Fatalf("ordered commands: %v %v", commands, err)
		}
	})
	t.Run("rollback when start insertion fails", func(t *testing.T) {
		w := createWorkload(t, r, devA, "rollback")
		v := createVersion(t, r, devA, w)
		_, err := db.migrator.Exec(ctx, `CREATE FUNCTION forge.fail_start() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END; $$;
            CREATE TRIGGER fail_start BEFORE INSERT ON forge.commands FOR EACH ROW EXECUTE FUNCTION forge.fail_start()`)
		if err != nil {
			t.Fatal(err)
		}
		before := count(t, db.migrator, "runs")
		_, err = r.SubmitRun(ctx, devA, "rollback-key", runPayload(w, v))
		requireError(t, err, ErrUnavailable)
		if count(t, db.migrator, "runs") != before {
			t.Fatal("partial acceptance persisted")
		}
		_, err = db.migrator.Exec(ctx, `DROP TRIGGER fail_start ON forge.commands; DROP FUNCTION forge.fail_start()`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.SubmitRun(ctx, devA, "rollback-key", runPayload(w, v)); err != nil {
			t.Fatal("rollback key leaked", err)
		}
	})
	t.Run("rerun shares key scope and retries before dependency checks", func(t *testing.T) {
		w := createWorkload(t, r, devA, "rerun")
		v := createVersion(t, r, devA, w)
		parent, err := r.SubmitRun(ctx, devA, "rerun-parent", runPayload(w, v))
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		confirm := func(context.Context, Run) error { calls++; return nil }
		_, err = r.Rerun(ctx, devA, parent.ID, "rerun-parent", confirm)
		requireError(t, err, ErrConflict)
		child, err := r.Rerun(ctx, devA, parent.ID, "rerun-child", confirm)
		if err != nil || child.RerunOf == nil || *child.RerunOf != parent.ID || child.ID == parent.ID || calls != 1 {
			t.Fatalf("rerun: %+v %v calls=%d", child, err, calls)
		}
		again, err := r.Rerun(ctx, devA, parent.ID, "rerun-child", func(context.Context, Run) error { t.Error("retry queried dependency"); return ErrUnavailable })
		if err != nil || again.ID != child.ID {
			t.Fatal("rerun retry", err)
		}
		_, err = r.Rerun(ctx, devA, parent.ID, "rerun-new", func(context.Context, Run) error { return ErrUnavailable })
		requireError(t, err, ErrUnavailable)
		_, err = r.Rerun(ctx, operator, parent.ID, "operator-rerun", confirm)
		requireError(t, err, ErrNotFound)
	})
	t.Run("keyset pages filter before limit and keep tie breakers", func(t *testing.T) {
		principal := Principal{devA.Issuer, "page-owner", "developer"}
		var workloads []Workload
		for _, name := range []string{"page-a", "page-b", "page-c"} {
			workloads = append(workloads, createWorkload(t, r, principal, name))
		}
		// Same creation instant exercises UUID tie breaking, not timing luck.
		_, err := db.migrator.Exec(ctx, `ALTER TABLE forge.workloads DISABLE TRIGGER workload_revision`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.migrator.Exec(ctx, `UPDATE forge.workloads SET created_at='2026-01-01T00:00:00Z' WHERE owner_id IN (SELECT id FROM forge.principals WHERE subject='page-owner')`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.migrator.Exec(ctx, `ALTER TABLE forge.workloads ENABLE TRIGGER workload_revision`)
		if err != nil {
			t.Fatal(err)
		}
		createWorkload(t, r, devB, "newer-foreign")
		first, err := r.ListWorkloads(ctx, principal, 2, nil)
		if err != nil || len(first) != 2 {
			t.Fatal(first, err)
		}
		createWorkload(t, r, principal, "inserted-after-first-page")
		second, err := r.ListWorkloads(ctx, principal, 2, &Position{first[1].CreatedAt, first[1].ID})
		if err != nil || len(second) != 1 {
			t.Fatal(second, err)
		}
		if first[0].ID == second[0].ID || first[1].ID == second[0].ID {
			t.Fatal("repeated page")
		}
		for _, w := range workloads {
			v := createVersion(t, r, principal, w)
			if _, err := r.SubmitRun(ctx, principal, "key-"+w.Name, runPayload(w, v)); err != nil {
				t.Fatal(err)
			}
		}
		runs, err := r.ListRuns(ctx, principal, RunFilter{Limit: 2})
		if err != nil || len(runs) != 2 {
			t.Fatal(runs, err)
		}
		more, err := r.ListRuns(ctx, principal, RunFilter{Limit: 2, After: &Position{runs[1].CreatedAt, runs[1].ID}})
		if err != nil || len(more) != 1 {
			t.Fatal(more, err)
		}
		filtered, err := r.ListRuns(ctx, principal, RunFilter{Limit: 20, WorkloadID: workloads[0].ID})
		if err != nil || len(filtered) != 1 {
			t.Fatal(filtered, err)
		}
		_, err = r.ListRuns(ctx, devB, RunFilter{Limit: 20, WorkloadID: workloads[0].ID})
		requireError(t, err, ErrNotFound)
	})
	t.Run("exact OIDC identity and maximum lengths", func(t *testing.T) {
		prefix := "https://identity.example/"
		long := Principal{prefix + strings.Repeat("a", 2048-len(prefix)), strings.Repeat("\u03b1", 255), "developer"}
		w := createWorkload(t, r, long, "long-owner")
		if _, err := r.GetWorkload(ctx, long, w.ID); err != nil {
			t.Fatal(err)
		}
		other := Principal{"https://other.example/realms/forge", devA.Subject, "developer"}
		owned := createWorkload(t, r, devA, "issuer-identity")
		_, err := r.GetWorkload(ctx, other, owned.ID)
		requireError(t, err, ErrNotFound)
		createWorkload(t, r, other, "issuer-identity")
	})
	t.Run("equivalent submissions reuse identity but changed sets conflict", func(t *testing.T) {
		w := createWorkload(t, r, devA, "semantic-key")
		v := createVersion(t, r, devA, w)
		input := runInput{w.ID, v.ID, "Question?", []string{"platform-brief", "architecture-notes"}}
		payload, _ := json.Marshal(input)
		first, err := r.SubmitRun(ctx, devA, "semantic-key", payload)
		if err != nil {
			t.Fatal(err)
		}
		input.DocumentIDs = []string{"architecture-notes", "platform-brief"}
		payload, _ = json.MarshalIndent(input, "", "  ")
		again, err := r.SubmitRun(ctx, devA, "semantic-key", payload)
		if err != nil || again.ID != first.ID {
			t.Fatal("semantic retry", err)
		}
		input.DocumentIDs = []string{"platform-brief"}
		payload, _ = json.Marshal(input)
		_, err = r.SubmitRun(ctx, devA, "semantic-key", payload)
		requireError(t, err, ErrConflict)
		other := createWorkload(t, r, devB, "semantic-key")
		otherVersion := createVersion(t, r, devB, other)
		if _, err := r.SubmitRun(ctx, devB, "semantic-key", runPayload(other, otherVersion)); err != nil {
			t.Fatal("other owner key blocked", err)
		}
	})
	t.Run("reopen repository retains records and commands", func(t *testing.T) {
		w := createWorkload(t, r, devA, "reopen")
		v := createVersion(t, r, devA, w)
		run, err := r.SubmitRun(ctx, devA, "reopen-key", runPayload(w, v))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.Cancel(ctx, devA, run.ID); err != nil {
			t.Fatal(err)
		}
		db.pool.Close()
		pool, err := pgxpool.NewWithConfig(ctx, db.runtimeConfig.Copy())
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		fresh, err := New(pool)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fresh.GetVersion(ctx, devA, w.ID, v.ID); err != nil {
			t.Fatal(err)
		}
		same, err := fresh.SubmitRun(ctx, devA, "reopen-key", runPayload(w, v))
		if err != nil || same.ID != run.ID {
			t.Fatal("lost idempotency", err)
		}
		commands, err := fresh.Commands(ctx, devA, run.ID)
		if err != nil || len(commands) != 2 {
			t.Fatal("lost durable commands", err)
		}
		db.pool = pool
		db.repo = fresh
	})
}

func TestPostgreSQLMigrationsAndPrivileges(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	t.Run("idempotent concurrent migrations", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, err := pgx.ConnectConfig(ctx, db.migratorConfig.Copy())
				if err == nil {
					defer c.Close(ctx)
					err = Migrate(ctx, c)
				}
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatal(err)
			}
		}
		if count(t, db.migrator, "schema_migrations") != 1 {
			t.Fatal("migration duplicated")
		}
	})
	t.Run("checksum mismatch fails closed", func(t *testing.T) {
		altered := fstest.MapFS{"migrations/0001_product_state.sql": {Data: []byte("SELECT 1;")}}
		if err := migrateFS(ctx, db.migrator, altered); err == nil {
			t.Fatal("modified migration accepted")
		}
	})
	t.Run("failed forward migration rolls back DDL and ledger", func(t *testing.T) {
		original, _ := migrations.ReadFile("migrations/0001_product_state.sql")
		files := fstest.MapFS{"migrations/0001_product_state.sql": {Data: original}, "migrations/0002_failure.sql": {Data: []byte("CREATE TABLE forge.rollback_probe(id integer); SELECT no_such_function();")}}
		if err := migrateFS(ctx, db.migrator, files); err == nil {
			t.Fatal("failed migration committed")
		}
		var exists bool
		err := db.migrator.QueryRow(ctx, `SELECT to_regclass('forge.rollback_probe') IS NOT NULL`).Scan(&exists)
		if err != nil || exists {
			t.Fatal("DDL rollback failed", err)
		}
		if count(t, db.migrator, "schema_migrations") != 1 {
			t.Fatal("failed migration ledger persisted")
		}
	})
	t.Run("runtime cannot migrate or change immutable records", func(t *testing.T) {
		c, err := db.pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		err = Migrate(ctx, c.Conn())
		c.Release()
		if err == nil {
			t.Fatal("runtime migrated")
		}
		for _, sql := range []string{`CREATE TABLE forge.forbidden(id integer)`, `SELECT * FROM forge.schema_migrations`, `DELETE FROM forge.workloads`, `UPDATE forge.principals SET subject='changed'`, `UPDATE forge.versions SET spec='{}'`, `UPDATE forge.runs SET workflow_id='replacement'`, `UPDATE forge.commands SET state='delivered'`} {
			if _, err := db.pool.Exec(ctx, sql); err == nil {
				t.Fatalf("runtime permitted %s", sql)
			}
		}
	})
}

func TestStoreGuards(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("nil pool accepted")
	}
	validator, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	r := &Repository{validator: validator}
	requireError(t, r.principal(Principal{"not-uri", "subject", "developer"}), contract.ErrInvalidInput)
	requireError(t, r.principal(Principal{devA.Issuer, "a", "untrusted"}), ErrForbidden)
	requireError(t, r.page(101, nil), contract.ErrInvalidInput)
	requireError(t, r.page(1, &Position{ID: "invalid"}), contract.ErrInvalidInput)
	for i := 0; i < 20; i++ {
		id, err := uuid()
		if err != nil {
			t.Fatal(err)
		}
		if err := r.id(id); err != nil {
			t.Fatal(err)
		}
	}
	requireError(t, storageError(context.Canceled), context.Canceled)
	requireError(t, storageError(errors.New("secret diagnostic")), ErrUnavailable)
}
