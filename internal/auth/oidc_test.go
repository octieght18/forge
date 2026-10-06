package auth_test

import (
	"context"
	"errors"
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
