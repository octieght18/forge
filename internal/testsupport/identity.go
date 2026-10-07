// Package testsupport provides signed-token and database fixtures for integration tests.
// It is never referenced by the API binary.
package testsupport

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

type Identity struct {
	Server      *httptest.Server
	mu          sync.Mutex
	key         *rsa.PrivateKey
	kid         string
	Unavailable bool
	Requests    int
}

func NewIdentity(t *testing.T) *Identity {
	return NewIdentityWithMiddleware(t, nil)
}

// NewIdentityWithMiddleware permits controlled network stalls before taking the
// fixture's signing-key mutex. Middleware must be installed before serving.
func NewIdentityWithMiddleware(t *testing.T, middleware func(http.Handler) http.Handler) *Identity {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	i := &Identity{key: key, kid: "first"}
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i.mu.Lock()
		defer i.mu.Unlock()
		i.Requests++
		if i.Unavailable {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": i.Server.URL, "jwks_uri": i.Server.URL + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &i.key.PublicKey, KeyID: i.kid, Algorithm: "RS256", Use: "sig"}}})
		default:
			w.WriteHeader(404)
		}
	})
	if middleware != nil {
		handler = middleware(handler)
	}
	i.Server = httptest.NewServer(handler)
	t.Cleanup(i.Server.Close)
	return i
}
func (i *Identity) SetUnavailable(value bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Unavailable = value
}
func (i *Identity) Rotate(t *testing.T) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.key = key
	i.kid = "rotated"
}
func (i *Identity) Token(t *testing.T, subject, role string, changes map[string]any) string {
	t.Helper()
	i.mu.Lock()
	defer i.mu.Unlock()
	now := time.Now().Unix()
	claims := map[string]any{"iss": i.Server.URL, "sub": subject, "aud": []string{"forge-api"}, "typ": "Bearer", "iat": now, "exp": now + 300, "resource_access": map[string]any{"forge-api": map[string]any{"roles": []string{role}}}}
	for key, value := range changes {
		claims[key] = value
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: i.key}, (&jose.SignerOptions{}).WithHeader("kid", i.kid).WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := signer.Sign(b)
	if err != nil {
		t.Fatal(err)
	}
	value, err := signed.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return value
}
