package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

type backendFunc func(context.Context, store.Delivery) (Receipt, error)

func (f backendFunc) Deliver(ctx context.Context, x store.Delivery) (Receipt, error) {
	return f(ctx, x)
}
func receipt(x store.Delivery) Receipt {
	return Receipt{x.Run.ID, x.Run.WorkflowID, x.Kind, x.Run.Owner}
}

func fixture(t *testing.T) (*testsupport.Database, *store.Dispatcher, *pgxpool.Pool) {
	t.Helper()
	db := testsupport.NewDatabase(t)
	pool := testsupport.DispatcherPool(t, db)
	queue, err := store.NewDispatcher(pool)
	if err != nil {
		t.Fatal(err)
	}
	return db, queue, pool
}
func submit(t *testing.T, db *testsupport.Database, subject string) (store.Run, store.Principal) {
	t.Helper()
	p := store.Principal{Issuer: "https://identity.example/realms/forge", Subject: subject, Role: "developer"}
	payload, _ := json.Marshal(map[string]any{"name": "research-" + subject, "description": "synthetic reconciliation fixture"})
	w, err := db.Repo.CreateWorkload(context.Background(), p, payload)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := contract.Examples.ReadFile("examples/create-version.json")
	v, err := db.Repo.CreateVersion(context.Background(), p, w.ID, version)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ = json.Marshal(map[string]any{"workload_id": w.ID, "version_id": v.ID, "question": "Which service owns execution?", "document_ids": []string{"platform-brief"}})
	run, err := db.Repo.SubmitRun(context.Background(), p, "reconcile-fixture", payload)
	if err != nil {
		t.Fatal(err)
	}
	return run, p
}
func newReconciler(t *testing.T, q Queue, b Backend) *Reconciler {
	t.Helper()
	r, err := New(q, b, Defaults())
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func expire(t *testing.T, db *testsupport.Database, id string) {
	t.Helper()
	if _, err := db.Migration.Exec(context.Background(), `UPDATE forge.commands SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
}

func TestPostgreSQLConcurrentClaimsAndOwnership(t *testing.T) {
	db, q, pool := fixture(t)
	ctx := context.Background()
	a, pa := submit(t, db, "a")
	b, _ := submit(t, db, "b")
	if _, err := db.Repo.Cancel(ctx, pa, a.ID); err != nil {
		t.Fatal(err)
	}
	results := make(chan *store.Delivery, 12)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Go(func() { x, err := q.Claim(ctx, 30*time.Second); results <- x; errs <- err })
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for x := range results {
		if x == nil {
			continue
		}
		if x.Kind != "start" || seen[x.Run.ID] {
			t.Fatal("duplicate or premature cancel claim")
		}
		seen[x.Run.ID] = true
		want := "a"
		if x.Run.ID == b.ID {
			want = "b"
		}
		if x.Run.Owner.Subject != want || x.Run.WorkflowID != "forge-run/"+x.Run.ID {
			t.Fatal("cross-owner envelope")
		}
	}
	if len(seen) != 2 || !seen[a.ID] || !seen[b.ID] {
		t.Fatal("lost eligible starts")
	}
	for _, sql := range []string{`INSERT INTO forge.commands(id,run_id,kind) SELECT gen_random_uuid(),id,'cancel' FROM forge.runs LIMIT 1`, `UPDATE forge.runs SET workflow_id='changed'`, `UPDATE forge.commands SET run_id=gen_random_uuid()`, `DELETE FROM forge.commands`, `SELECT * FROM forge.schema_migrations`} {
		if _, err := pool.Exec(ctx, sql); err == nil {
			t.Fatalf("dispatcher exceeded grants: %s", sql)
		}
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE forge.commands SET attempts=attempts+1`); err == nil {
		t.Fatal("API role acquired delivery permission")
	}
	if _, err := db.Pool.Exec(ctx, `INSERT INTO forge.commands(id,run_id,kind,state,delivered_at)
 SELECT gen_random_uuid(),id,'start','delivered',clock_timestamp() FROM forge.runs LIMIT 1`); err == nil {
		t.Fatal("API producer manufactured delivery acknowledgement")
	}
}

func TestPostgreSQLExpiredLeaseFencing(t *testing.T) {
	db, q, _ := fixture(t)
	submit(t, db, "fence")
	ctx := context.Background()
	old, err := q.Claim(ctx, 30*time.Second)
	if err != nil || old == nil {
		t.Fatal(err)
	}
	if x, err := q.Claim(ctx, 30*time.Second); err != nil || x != nil {
		t.Fatal("active lease reclaimed", err)
	}
	expire(t, db, old.CommandID)
	if err := q.Acknowledge(ctx, *old); !errors.Is(err, store.ErrLeaseLost) {
		t.Fatal("expired lease acknowledged", err)
	}
	fresh, err := q.Claim(ctx, 30*time.Second)
	if err != nil || fresh == nil || fresh.Token == old.Token || fresh.Attempt != 2 {
		t.Fatal("lease not recovered", err)
	}
	for _, err := range []error{q.Acknowledge(ctx, *old), q.Fail(ctx, *old, "retryable")} {
		if !errors.Is(err, store.ErrLeaseLost) {
			t.Fatal("stale claimant wrote", err)
		}
	}
	if err := q.Acknowledge(ctx, *fresh); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migration.Exec(ctx, `UPDATE forge.commands SET state='pending',delivered_at=NULL WHERE id=$1`, fresh.CommandID); err == nil {
		t.Fatal("delivered command moved backwards")
	}
}

func TestPostgreSQLLockedRowsAndCancelGuard(t *testing.T) {
	db, q, pool := fixture(t)
	ctx := context.Background()
	a, pa := submit(t, db, "locked")
	b, _ := submit(t, db, "available")
	cancel, err := db.Repo.Cancel(ctx, pa, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migration.Exec(ctx, `UPDATE forge.commands SET run_id=$1 WHERE id=$2`, b.ID, cancel.ID); err == nil {
		t.Fatal("command ownership reference changed")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT id FROM forge.commands WHERE run_id=$1 AND kind='start' FOR UPDATE`, a.ID); err != nil {
		t.Fatal(err)
	}
	bounded, end := context.WithTimeout(ctx, time.Second)
	defer end()
	x, err := q.Claim(bounded, 30*time.Second)
	if err != nil || x == nil || x.Run.ID != b.ID {
		t.Fatal("locked row blocked another command", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	// Even a manually claimed cancel cannot bypass the SQL acknowledgement guard.
	token := "77777777-7777-4777-8777-777777777777"
	if _, err := db.Migration.Exec(ctx, `UPDATE forge.commands SET attempts=1,lease_token=$1,lease_until=clock_timestamp()+interval '30 seconds' WHERE id=$2`, token, cancel.ID); err != nil {
		t.Fatal(err)
	}
	if err := q.Acknowledge(ctx, store.Delivery{CommandID: cancel.ID, Token: token}); err == nil {
		t.Fatal("cancel acknowledged before start")
	}
}

type lostAck struct{ Queue }

func (lostAck) Acknowledge(context.Context, store.Delivery) error { return store.ErrUnavailable }

func TestPostgreSQLAmbiguousStartAndCancelRecovery(t *testing.T) {
	db, q, pool := fixture(t)
	run, p := submit(t, db, "crash")
	ctx := context.Background()
	if _, err := db.Repo.Cancel(ctx, p, run.ID); err != nil {
		t.Fatal(err)
	}
	created, calls, cancels := 0, 0, 0
	identities := map[string]Receipt{}
	b := backendFunc(func(_ context.Context, x store.Delivery) (Receipt, error) {
		if x.Kind == "start" {
			calls++
			if old, exists := identities[x.Run.WorkflowID]; exists {
				return old, nil
			}
			created++
			identities[x.Run.WorkflowID] = receipt(x)
		} else {
			if _, exists := identities[x.Run.WorkflowID]; !exists {
				t.Fatal("cancel before start")
			}
			cancels++
		}
		return receipt(x), nil
	})
	r := newReconciler(t, lostAck{q}, b)
	if _, err := r.Step(ctx); !errors.Is(err, store.ErrUnavailable) {
		t.Fatal("ack loss not injected", err)
	}
	var id string
	if err := db.Migration.QueryRow(ctx, `SELECT id::text FROM forge.commands WHERE run_id=$1 AND kind='start'`, run.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if x, err := q.Claim(ctx, 30*time.Second); err != nil || x != nil {
		t.Fatal("cancel bypassed pending start", err)
	}
	expire(t, db, id)
	// Recreate a pool/reconciler after the injected crash window. The backend's
	// durable identity lookup remains available; only the client is restarted.
	freshPool, err := pgxpool.NewWithConfig(ctx, pool.Config().Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer freshPool.Close()
	fresh, err := store.NewDispatcher(freshPool)
	if err != nil {
		t.Fatal(err)
	}
	r = newReconciler(t, fresh, b)
	for i := 0; i < 2; i++ {
		if handled, err := r.Step(ctx); err != nil || !handled {
			t.Fatal("recovery failed", err)
		}
	}
	if created != 1 || calls != 2 || cancels != 1 {
		t.Fatalf("duplicate effect or cancel loss: %d/%d/%d", created, calls, cancels)
	}
	commands, err := db.Repo.Commands(ctx, p, run.ID)
	if err != nil || len(commands) != 2 {
		t.Fatal(err)
	}
	for _, c := range commands {
		if c.State != "delivered" || c.DeliveredAt == nil {
			t.Fatal("acknowledgement lost")
		}
	}
}

func TestPostgreSQLRetryExhaustionAndBackoff(t *testing.T) {
	db, q, _ := fixture(t)
	run, _ := submit(t, db, "retry")
	ctx := context.Background()
	calls := 0
	r := newReconciler(t, q, backendFunc(func(context.Context, store.Delivery) (Receipt, error) {
		calls++
		return Receipt{}, fmt.Errorf("unpersisted provider detail: %w", ErrRetryable)
	}))
	for attempt := 1; attempt <= 8; attempt++ {
		if handled, err := r.Step(ctx); err != nil || !handled {
			t.Fatal("retry step failed", err)
		}
		var count int
		var blocked bool
		var code string
		var remaining float64
		if err := db.Migration.QueryRow(ctx, `SELECT attempts,blocked,last_error_code,extract(epoch FROM next_attempt_at-clock_timestamp()) FROM forge.commands WHERE run_id=$1`, run.ID).Scan(&count, &blocked, &code, &remaining); err != nil {
			t.Fatal(err)
		}
		if count != attempt || blocked != (attempt == 8) {
			t.Fatal("attempt accounting lost")
		}
		want := float64(min(60, 1<<(attempt-1)))
		if remaining > want || remaining < want-1 {
			t.Fatalf("wrong durable backoff %.3f want %.0f", remaining, want)
		}
		if attempt < 8 && code != "retryable" || attempt == 8 && code != "attempts_exhausted" {
			t.Fatal("unsafe error code", code)
		}
		if x, err := q.Claim(ctx, 30*time.Second); err != nil || x != nil {
			t.Fatal("backoff ignored", err)
		}
		if _, err := db.Migration.Exec(ctx, `UPDATE forge.commands SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE run_id=$1`, run.ID); err != nil {
			t.Fatal(err)
		}
	}
	if handled, err := r.Step(ctx); err != nil || handled || calls != 8 {
		t.Fatal("parked command retried", err)
	}
}

func TestPostgreSQLCrashedFinalAttemptIsParked(t *testing.T) {
	db, q, _ := fixture(t)
	run, _ := submit(t, db, "last")
	ctx := context.Background()
	if _, err := db.Migration.Exec(ctx, `UPDATE forge.commands SET attempts=7 WHERE run_id=$1`, run.ID); err != nil {
		t.Fatal(err)
	}
	x, err := q.Claim(ctx, 30*time.Second)
	if err != nil || x == nil || x.Attempt != 8 {
		t.Fatal(err)
	}
	expire(t, db, x.CommandID)
	if next, err := q.Claim(ctx, 30*time.Second); err != nil || next != nil {
		t.Fatal("ninth attempt claimed", err)
	}
	var blocked bool
	var code string
	if err := db.Migration.QueryRow(ctx, `SELECT blocked,last_error_code FROM forge.commands WHERE id=$1`, x.CommandID).Scan(&blocked, &code); err != nil || !blocked || code != "attempts_exhausted" {
		t.Fatal("crashed attempt not parked", err)
	}
}

func TestPostgreSQLIdentityAndPermanentFailureAreParked(t *testing.T) {
	for _, mode := range []string{"wrong-owner", "wrong-run", "wrong-kind", "wrong-workflow", "permanent"} {
		t.Run(mode, func(t *testing.T) {
			db, q, _ := fixture(t)
			run, _ := submit(t, db, "receipt")
			ctx := context.Background()
			r := newReconciler(t, q, backendFunc(func(_ context.Context, x store.Delivery) (Receipt, error) {
				rc := receipt(x)
				switch mode {
				case "wrong-owner":
					rc.Owner.Subject = "other"
				case "wrong-run":
					rc.RunID = "other"
				case "wrong-kind":
					rc.Kind = "cancel"
				case "wrong-workflow":
					rc.WorkflowID = "other"
				default:
					return Receipt{}, errors.New("unpersisted sensitive error")
				}
				return rc, nil
			}))
			if handled, err := r.Step(ctx); err != nil || !handled {
				t.Fatal(err)
			}
			var blocked bool
			var code, state string
			if err := db.Migration.QueryRow(ctx, `SELECT blocked,last_error_code,state FROM forge.commands WHERE run_id=$1`, run.ID).Scan(&blocked, &code, &state); err != nil {
				t.Fatal(err)
			}
			want := "identity_mismatch"
			if mode == "permanent" {
				want = "permanent"
			}
			if !blocked || code != want || state != "pending" {
				t.Fatal("unsafe acknowledgement")
			}
		})
	}
}
