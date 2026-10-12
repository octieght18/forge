// Package auth validates Keycloak access tokens; it never accepts identity headers.
package auth

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/octieght18/forge/internal/store"
)

var ErrUnauthenticated = errors.New("invalid access token")

// Kind separates a person, a service, and a workload. Only a user token is a
// product principal. A workload is taken from the run record, not from a token.
type Kind string

const (
	KindUser     Kind = "user"
	KindService  Kind = "service"
	KindWorkload Kind = "workload"
)

// Identity is the verified caller. Workload is set only for KindWorkload.
type Identity struct {
	Kind      Kind
	Principal store.Principal
	Workload  Workload
}

// Workload is the execution identity copied from a product run.
type Workload struct {
	RunID, WorkflowID, Tenant, Project, Issuer, Subject string
}

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
		Kind         string         `json:"identity_kind"`
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
	roles := map[string]bool{}
	for _, candidate := range claims.Access[v.audience].Roles {
		roles[candidate] = true
	}
	kind := claims.Kind
	if kind == "" {
		kind = string(KindUser)
	}
	switch Kind(kind) {
	case KindUser:
		role := ""
		if roles["operator"] {
			role = "operator"
		} else if roles["developer"] {
			role = "developer"
		}
		if role == "" || roles["service"] {
			return store.Principal{}, store.ErrForbidden
		}
		return store.Principal{Issuer: token.Issuer, Subject: token.Subject, Role: role}, nil
	case KindService:
		if !roles["service"] || roles["developer"] || roles["operator"] {
			return store.Principal{}, store.ErrForbidden
		}
		return store.Principal{}, ErrNotUser
	case KindWorkload:
		return store.Principal{}, ErrUnauthenticated
	default:
		return store.Principal{}, ErrUnauthenticated
	}
}

// ErrNotUser means the token verified but it is a service identity, not a person.
var ErrNotUser = errors.New("service identity is not a user")

// Classify reports the token kind. Product routes use Authenticate, which accepts only users.
func (v *Verifier) Classify(ctx context.Context, r *http.Request) (Identity, error) {
	principal, err := v.Authenticate(ctx, r)
	if err == nil {
		return Identity{Kind: KindUser, Principal: principal}, nil
	}
	if !errors.Is(err, ErrNotUser) {
		return Identity{}, err
	}
	headers := r.Header.Values("Authorization")
	parts := strings.Fields(headers[0])
	token, verifyErr := v.verifier.Verify(ctx, parts[1])
	if verifyErr != nil || token.Subject == "" {
		return Identity{}, ErrUnauthenticated
	}
	return Identity{Kind: KindService, Principal: store.Principal{Issuer: token.Issuer, Subject: token.Subject}}, nil
}

// WorkloadFromRun builds the execution identity from the stored run. It does not
// accept a bearer token.
var resourceID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func WorkloadFromRun(runID, workflowID, tenant, project, issuer, subject string) (Workload, error) {
	if !resourceID.MatchString(runID) || workflowID != "forge-run/"+runID || issuer == "" || subject == "" {
		return Workload{}, ErrUnauthenticated
	}
	if tenant == "" {
		tenant = store.DemoTenant
	}
	if project == "" {
		project = store.DemoProject
	}
	if !resourceID.MatchString(tenant) || !resourceID.MatchString(project) {
		return Workload{}, ErrUnauthenticated
	}
	return Workload{RunID: runID, WorkflowID: workflowID, Tenant: tenant, Project: project, Issuer: issuer, Subject: subject}, nil
}
