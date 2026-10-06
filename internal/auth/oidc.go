// Package auth validates Keycloak access tokens; it never accepts identity headers.
package auth

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/octieght18/forge/internal/store"
)

var ErrUnauthenticated = errors.New("invalid access token")

type Verifier struct {
	verifier         *oidc.IDTokenVerifier
	issuer, audience string
}

func trustedURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback())
}

type limitedTransport struct{ base http.RoundTripper }
type limitedBody struct {
	io.Reader
	io.Closer
}

func (t limitedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	response.Body = limitedBody{io.LimitReader(response.Body, 1<<20), response.Body}
	return response, nil
}

// New performs discovery from the operator's issuer. Same-origin JWKS, no
// redirects, bounded responses/timeouts and RS256-only verification are enforced.
func New(ctx context.Context, issuer, audience string) (*Verifier, error) {
	if !trustedURL(issuer) || audience == "" || utf8.RuneCountInString(issuer) > 2048 {
		return nil, errors.New("invalid OIDC configuration")
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: limitedTransport{http.DefaultTransport}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("OIDC redirects disabled") }}
	ctx = oidc.ClientContext(ctx, client)
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, errors.New("OIDC discovery failed")
	}
	var metadata struct {
		JWKS string `json:"jwks_uri"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return nil, errors.New("invalid OIDC metadata")
	}
	origin, _ := url.Parse(issuer)
	keys, err := url.Parse(metadata.JWKS)
	if err != nil || !trustedURL(metadata.JWKS) || keys.Scheme != origin.Scheme || keys.Host != origin.Host {
		return nil, errors.New("untrusted OIDC key endpoint")
	}
	return &Verifier{provider.VerifierContext(ctx, &oidc.Config{ClientID: audience, SupportedSigningAlgs: []string{oidc.RS256}}), issuer, audience}, nil
}

func (v *Verifier) Authenticate(ctx context.Context, r *http.Request) (store.Principal, error) {
	headers := r.Header.Values("Authorization")
	if len(headers) != 1 || len(headers[0]) > 16384 {
		return store.Principal{}, ErrUnauthenticated
	}
	parts := strings.Fields(headers[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return store.Principal{}, ErrUnauthenticated
	}
	token, err := v.verifier.Verify(ctx, parts[1])
	if err != nil {
		return store.Principal{}, ErrUnauthenticated
	}
	var claims struct {
		Type         string         `json:"typ"`
		IssuedAt     int64          `json:"iat"`
		Expires      int64          `json:"exp"`
		NotBefore    int64          `json:"nbf"`
		Confirmation map[string]any `json:"cnf"`
		Access       map[string]struct {
			Roles []string `json:"roles"`
		} `json:"resource_access"`
	}
	if token.Claims(&claims) != nil || token.Issuer != v.issuer || token.Subject == "" || utf8.RuneCountInString(token.Subject) > 255 || strings.ContainsRune(token.Subject, 0) || claims.Type != "Bearer" || len(claims.Confirmation) != 0 {
		return store.Principal{}, ErrUnauthenticated
	}
	now := time.Now().Unix()
	if claims.IssuedAt <= 0 || claims.IssuedAt > now || claims.NotBefore > now || claims.Expires <= now || claims.Expires <= claims.IssuedAt || claims.Expires-claims.IssuedAt > 300 {
		return store.Principal{}, ErrUnauthenticated
	}
	role := ""
	for _, candidate := range claims.Access[v.audience].Roles {
		if candidate == "operator" {
			role = "operator"
			break
		}
		if candidate == "developer" {
			role = "developer"
		}
	}
	if role == "" {
		return store.Principal{}, store.ErrForbidden
	}
	return store.Principal{Issuer: token.Issuer, Subject: token.Subject, Role: role}, nil
}
