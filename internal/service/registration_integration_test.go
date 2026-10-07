package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/config"
	"github.com/octieght18/forge/internal/httpapi"
	"github.com/octieght18/forge/internal/service"
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

type registrationFixture struct {
	db            *testsupport.Database
	url, token    string
	client        *http.Client
	stop          context.CancelFunc
	done          chan struct{}
	serveErr      error // published by closing done
	ready         *httpapi.Readiness
	patchFinished chan error
}

func newRegistrationFixture(t *testing.T, grace time.Duration, middleware ...func(http.Handler) http.Handler) *registrationFixture {
	t.Helper()
	db := testsupport.NewDatabase(t)
	var wrap func(http.Handler) http.Handler
	if len(middleware) > 0 {
		wrap = middleware[0]
	}
	issuer := testsupport.NewIdentityWithMiddleware(t, wrap)
	v, err := auth.New(context.Background(), issuer.Server.URL, "forge-api")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := httpapi.LoadCorpusPolicy([]byte(`{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":["platform-brief","architecture-notes"]}`))
	if err != nil {
		t.Fatal(err)
	}
	f := &registrationFixture{db: db, token: issuer.Token(t, "f10-owner", "developer", nil), client: &http.Client{Timeout: 10 * time.Second}, done: make(chan struct{}), ready: &httpapi.Readiness{}, patchFinished: make(chan error, 1)}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h, err := httpapi.NewRegistrationHandler(logger, f.ready, httpapi.Registration{Repository: db.Repo, Auth: v, Pool: db.Pool, Policy: policy, CursorKey: bytes.Repeat([]byte{3}, 32), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pprof.Do(r.Context(), pprof.Labels("component", "api"), func(ctx context.Context) { h.ServeHTTP(w, r.WithContext(ctx)) })
		if r.Method == "PATCH" {
			f.patchFinished <- r.Context().Err()
		}
	})
	c, err := config.Load(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	c.ShutdownTimeout = grace
	s, err := service.New(c, logger, f.ready, handler)
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.url = "http://" + l.Addr().String()
	ctx, stop := context.WithCancel(context.Background())
	f.stop = stop
	go func() { f.serveErr = s.Run(ctx, l); close(f.done) }()
	t.Cleanup(func() { stop(); waitSignal(t, f.done); f.client.CloseIdleConnections() })
	return f
}

func TestRegistrationJWKSHTTPDisconnect(t *testing.T) {
	started, released := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f := newRegistrationFixture(t, time.Second, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/keys" {
				once.Do(func() { close(started) })
				select {
				case <-released:
				case <-r.Context().Done():
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	})
	defer close(released)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := f.request(ctx, "PATCH", "/api/v1/workloads/11111111-1111-4111-8111-111111111111", `{"description":"cancel"}`, "")
		done <- err
	}()
	waitSignal(t, started)
	cancel()
	if err := waitSignal(t, done); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := waitSignal(t, f.patchFinished); !errors.Is(err, context.Canceled) {
		t.Fatalf("HTTP context=%v", err)
	}
	if f.db.Pool.Stat().AcquiredConns() != 0 {
		t.Fatal("unauthenticated request reached database")
	}
}

func waitSignal[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("synchronization deadline exceeded")
	}
	var zero T
	return zero
}

func eventually(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("condition did not become true")
		case <-tick.C:
		}
	}
}

func (f *registrationFixture) request(ctx context.Context, method, path, body, tag string) (int, []byte, error) {
	r, err := http.NewRequestWithContext(ctx, method, f.url+path, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	r.Header.Set("Authorization", "Bearer "+f.token)
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if tag != "" {
		r.Header.Set("If-Match", tag)
	}
	response, err := f.client.Do(r)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	return response.StatusCode, payload, err
}

// An AFTER UPDATE trigger blocks only after the row has actually been changed
// inside the API transaction. Cancellation must undo that change, not merely
// abandon a SELECT that never reached a mutation.
func blockUpdatedRow(t *testing.T, f *registrationFixture) func() {
	t.Helper()
	ctx := context.Background()
	key := int64(f.db.Migration.PgConn().PID())
	sql := fmt.Sprintf(`CREATE FUNCTION forge.f10_block_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(%d::bigint); RETURN NEW; END $$; CREATE TRIGGER f10_block_update AFTER UPDATE ON forge.workloads FOR EACH ROW EXECUTE FUNCTION forge.f10_block_update()`, key)
	if _, err := f.db.Migration.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Migration.Exec(ctx, `SELECT pg_advisory_lock($1::bigint)`, key); err != nil {
		t.Fatal(err)
	}
	release := func() {
		if _, err := f.db.Migration.Exec(ctx, `SELECT pg_advisory_unlock($1::bigint)`, key); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(release)
	return release
}

func blockedCount(t *testing.T, f *registrationFixture) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var n int
	err := f.db.Admin.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE $1::int=ANY(pg_blocking_pids(pid))`, f.db.Migration.PgConn().PID()).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRegistrationCancellationAndDrain(t *testing.T) {
	for _, mode := range []string{"client-cancel", "graceful-drain", "grace-expiry"} {
		t.Run(mode, func(t *testing.T) {
			grace := 2 * time.Second
			if mode == "grace-expiry" {
				grace = 100 * time.Millisecond
			}
			f := newRegistrationFixture(t, grace)
			status, body, err := f.request(context.Background(), "POST", "/api/v1/workloads", `{"name":"cancellation-fixture","description":"original"}`, "")
			if err != nil || status != 201 {
				t.Fatalf("seed status=%d error=%v", status, err)
			}
			var original store.Workload
			if err := json.Unmarshal(body, &original); err != nil {
				t.Fatal(err)
			}
			release := blockUpdatedRow(t, f)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type result struct {
				status int
				err    error
			}
			response := make(chan result, 1)
			path := "/api/v1/workloads/" + original.ID
			go func() {
				status, _, err := f.request(ctx, "PATCH", path, `{"description":"changed"}`, fmt.Sprintf(`"%s:1"`, original.ID))
				response <- result{status, err}
			}()
			eventually(t, func() bool { return blockedCount(t, f) == 1 })
			switch mode {
			case "client-cancel":
				cancel()
				if got := waitSignal(t, response); !errors.Is(got.err, context.Canceled) {
					t.Fatalf("client error=%v", got.err)
				}
				if err := waitSignal(t, f.patchFinished); !errors.Is(err, context.Canceled) {
					t.Fatalf("handler context=%v", err)
				}
			case "graceful-drain":
				f.stop()
				eventually(t, func() bool { return !f.ready.Ready() })
				if blockedCount(t, f) != 1 {
					t.Fatal("drain canceled dependency before grace expired")
				}
				release()
				if got := waitSignal(t, response); got.err != nil || got.status != 200 {
					t.Fatalf("drain response=%+v", got)
				}
				if err := waitSignal(t, f.patchFinished); err != nil {
					t.Fatalf("drained handler canceled: %v", err)
				}
				waitSignal(t, f.done)
				if f.serveErr != nil {
					t.Fatal(f.serveErr)
				}
			case "grace-expiry":
				f.stop()
				waitSignal(t, f.done)
				if !errors.Is(f.serveErr, context.DeadlineExceeded) {
					t.Fatalf("shutdown error=%v", f.serveErr)
				}
				waitSignal(t, response) // connection close may race with a 503 response
				if err := waitSignal(t, f.patchFinished); !errors.Is(err, context.Canceled) {
					t.Fatalf("handler context=%v", err)
				}
			}
			eventually(t, func() bool { return blockedCount(t, f) == 0 && f.db.Pool.Stat().AcquiredConns() == 0 })
			release()
			ctx, done := context.WithTimeout(context.Background(), time.Second)
			defer done()
			var revision int64
			var description string
			if err := f.db.Pool.QueryRow(ctx, `SELECT revision,description FROM forge.workloads WHERE id=$1`, original.ID).Scan(&revision, &description); err != nil {
				t.Fatal(err)
			}
			wantRevision, wantDescription := int64(1), "original"
			if mode == "graceful-drain" {
				wantRevision, wantDescription = 2, "changed"
			}
			if revision != wantRevision || description != wantDescription {
				t.Fatalf("stored revision=%d description=%s", revision, description)
			}
			if mode == "client-cancel" {
				status, _, err := f.request(ctx, "GET", path, "", "")
				if err != nil || status != 200 {
					t.Fatalf("HTTP recovery status=%d error=%v", status, err)
				}
			}
		})
	}
}
