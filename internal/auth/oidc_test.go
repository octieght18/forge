package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestAccessTokensAndKeyRotation(t *testing.T) {
	issuer := testsupport.NewIdentity(t)
	v, err := auth.New(context.Background(), issuer.Server.URL, "forge-api")
	if err != nil {
		t.Fatal(err)
	}
	check := func(token string) error {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		_, err := v.Authenticate(r.Context(), r)
		return err
	}
	good := issuer.Token(t, "developer-a", "developer", nil)
	if err := check(good); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	cases := map[string]map[string]any{"wrong issuer": {"iss": "https://evil.example"}, "wrong audience": {"aud": "other"}, "expired": {"iat": now - 300, "exp": now - 1}, "ID token": {"typ": "ID"}, "missing type": {"typ": nil}, "future not before": {"nbf": now + 60}, "future issued": {"iat": now + 30}, "too long lived": {"iat": now, "exp": now + 301}, "empty subject": {"sub": ""}, "bound token": {"cnf": map[string]string{"jkt": "bound"}}}
	for name, changes := range cases {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(check(issuer.Token(t, "a", "developer", changes)), auth.ErrUnauthenticated) {
				t.Fatal("invalid token accepted")
			}
		})
	}
	if !errors.Is(check(issuer.Token(t, "a", "admin", nil)), store.ErrForbidden) {
		t.Fatal("unknown role accepted")
	}
	serviceToken := issuer.Token(t, "dispatcher", "service", map[string]any{"identity_kind": "service"})
	if !errors.Is(check(serviceToken), auth.ErrNotUser) {
		t.Fatal("service token accepted as a user")
	}
	classified, err := v.Classify(context.Background(), requestWith(serviceToken))
	if err != nil || classified.Kind != auth.KindService || classified.Principal.Subject != "dispatcher" || classified.Principal.Role != "" {
		t.Fatal(err, classified)
	}
	user, err := v.Classify(context.Background(), requestWith(good))
	if err != nil || user.Kind != auth.KindUser || user.Principal.Role != "developer" {
		t.Fatal(err, user)
	}
	if !errors.Is(check(issuer.Token(t, "workload", "developer", map[string]any{"identity_kind": "workload"})), auth.ErrUnauthenticated) {
		t.Fatal("workload token accepted")
	}
	workload, err := auth.WorkloadFromRun("33333333-3333-4333-8333-333333333333", "forge-run/33333333-3333-4333-8333-333333333333", "", "", "https://issuer.example", "owner")
	if err != nil || workload.Tenant != store.DemoTenant || workload.WorkflowID != "forge-run/33333333-3333-4333-8333-333333333333" {
		t.Fatal(err, workload)
	}
	if _, err = auth.WorkloadFromRun("33333333-3333-4333-8333-333333333333", "other", "", "", "https://issuer.example", "owner"); err == nil {
		t.Fatal("mismatched workflow accepted")
	}
	forged := testsupport.NewIdentity(t).Token(t, "a", "developer", map[string]any{"iss": issuer.Server.URL})
	if !errors.Is(check(forged), auth.ErrUnauthenticated) {
		t.Fatal("untrusted signer accepted")
	}
	issuer.SetUnavailable(true)
	if err := check(good); err != nil {
		t.Fatal("cached key unavailable", err)
	}
	issuer.SetUnavailable(false)
	issuer.Rotate(t)
	if err := check(issuer.Token(t, "a", "developer", nil)); err != nil {
		t.Fatal("key rotation failed", err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Add("Authorization", "Bearer "+good)
	r.Header.Add("Authorization", "Bearer "+good)
	if _, err := v.Authenticate(r.Context(), r); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatal("multiple authorization headers accepted")
	}
	if _, err := auth.New(context.Background(), "http://untrusted.example/realm", "forge-api"); err == nil {
		t.Fatal("nonlocal plaintext issuer accepted")
	}
}

func requestWith(token string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}
