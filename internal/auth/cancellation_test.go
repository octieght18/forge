package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/testsupport"
)

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(4 * time.Second):
		t.Fatal("OIDC synchronization deadline exceeded")
	}
	var zero T
	return zero
}

func TestDiscoveryCancellation(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	issuer := testsupport.NewIdentityWithMiddleware(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
			close(canceled)
		})
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := auth.New(ctx, issuer.Server.URL, "forge-api"); done <- err }()
	receive(t, started)
	cancel()
	if receive(t, done) == nil {
		t.Fatal("canceled discovery succeeded")
	}
	receive(t, canceled)
}

func TestJWKSCallerCancellationAndSharedFetch(t *testing.T) {
	for _, mode := range []string{"another-caller-recovers", "shared-fetch-times-out"} {
		t.Run(mode, func(t *testing.T) {
			started, released, upstreamCanceled := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			issuer := testsupport.NewIdentityWithMiddleware(t, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/keys" {
						once.Do(func() { close(started) })
						select {
						case <-released:
						case <-r.Context().Done():
							close(upstreamCanceled)
							return
						}
					}
					next.ServeHTTP(w, r)
				})
			})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(released) }) }
			defer release()
			startup, finishStartup := context.WithCancel(context.Background())
			v, err := auth.New(startup, issuer.Server.URL, "forge-api")
			finishStartup()
			if err != nil {
				t.Fatal(err)
			}
			token := issuer.Token(t, "owner", "developer", nil)
			verify := func(ctx context.Context) error {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Set("Authorization", "Bearer "+token)
				_, err := v.Authenticate(ctx, r)
				return err
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- verify(ctx) }()
			receive(t, started)
			cancel()
			if err := receive(t, done); !errors.Is(err, auth.ErrUnauthenticated) {
				t.Fatalf("canceled verification=%v", err)
			}
			if mode == "another-caller-recovers" {
				// The canceled startup/caller contexts must not poison the retained
				// key set or a subsequent caller using its own live context.
				go func() { done <- verify(context.Background()) }()
				release()
				if err := receive(t, done); err != nil {
					t.Fatal(err)
				}
			} else {
				// go-oidc coalesces key requests independently of any one caller.
				// Forge's two-second HTTP timeout still terminates that shared fetch.
				receive(t, upstreamCanceled)
				release()
				if err := verify(context.Background()); err != nil {
					t.Fatal("recovery", err)
				}
			}
		})
	}
}
