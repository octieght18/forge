// Package login implements the local operator's browser/PKCE token helper.
package login

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/octieght18/forge/internal/auth"
	"golang.org/x/oauth2"
)

type Helper struct {
	mu                                          sync.Mutex
	config                                      oauth2.Config
	identity                                    *auth.Verifier
	file, host, state, cookie, verifier, issuer string
	used                                        bool
	success                                     bool
	done                                        chan struct{}
}

// New requires a private existing output directory. The helper never prints a token.
func New(ctx context.Context, issuer, callback, file string) (*Helper, error) {
	v, err := auth.New(ctx, issuer, "forge-api")
	if err != nil {
		return nil, err
	}
	u, parseErr := url.Parse(callback)
	if parseErr != nil {
		return nil, errors.New("loopback callback required")
	}
	port, portErr := strconv.Atoi(u.Port())
	if u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.Path != "/callback" || u.RawQuery != "" || u.Fragment != "" || portErr != nil || port < 1 || port > 65535 {
		return nil, errors.New("loopback callback required")
	}
	parent, err := os.Lstat(filepath.Dir(file))
	if err != nil || !parent.IsDir() || runtime.GOOS != "windows" && parent.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private token directory required")
	}
	return &Helper{identity: v, issuer: issuer, file: file, host: u.Host, done: make(chan struct{}), config: oauth2.Config{ClientID: "forge-local-login", RedirectURL: callback, Scopes: []string{"openid"}, Endpoint: oauth2.Endpoint{AuthURL: issuer + "/protocol/openid-connect/auth", TokenURL: issuer + "/protocol/openid-connect/token", AuthStyle: oauth2.AuthStyleInParams}}}, nil
}

func (h *Helper) Done() <-chan struct{} { return h.done }
func (h *Helper) Succeeded() bool       { h.mu.Lock(); defer h.mu.Unlock(); return h.success }
func random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func equal(a, b string) bool { return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

func (h *Helper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	if r.Host != h.host {
		http.Error(w, "Invalid host", 400)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.used {
		http.Error(w, "Login helper finished; start another login", 410)
		return
	}
	switch r.URL.Path {
	case "/login":
		if h.state != "" {
			http.Error(w, "A login is already in progress", 409)
			return
		}
		h.state, h.cookie, h.verifier = random(), random(), oauth2.GenerateVerifier()
		http.SetCookie(w, &http.Cookie{Name: "forge-login", Value: h.cookie, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 300})
		http.Redirect(w, r, h.config.AuthCodeURL(h.state, oauth2.S256ChallengeOption(h.verifier), oauth2.SetAuthURLParam("prompt", "login")), 302)
	case "/callback":
		q, err := query(r)
		cookie, cerr := r.Cookie("forge-login")
		if err != nil || cerr != nil || !equal(q["state"], h.state) || !equal(cookie.Value, h.cookie) || q["iss"] != "" && q["iss"] != h.issuer {
			http.Error(w, "Invalid login callback", 400)
			return
		}
		// Consume the state before exchange: a code cannot be replayed on this helper.
		h.used = true
		defer close(h.done)
		http.SetCookie(w, &http.Cookie{Name: "forge-login", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		if q["error"] != "" || q["code"] == "" {
			http.Error(w, "Login was not completed", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect disabled") }}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
		token, err := h.config.Exchange(ctx, q["code"], oauth2.VerifierOption(h.verifier))
		if err != nil || token.AccessToken == "" || len(token.AccessToken) > 16384 {
			http.Error(w, "Token exchange failed", 502)
			return
		}
		check, _ := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1", nil)
		check.Header.Set("Authorization", "Bearer "+token.AccessToken)
		p, err := h.identity.Authenticate(ctx, check)
		if err != nil {
			http.Error(w, "API access token rejected", 403)
			return
		}
		payload, err := json.Marshal(struct {
			AccessToken string    `json:"access_token"`
			ExpiresAt   time.Time `json:"expires_at"`
			Issuer      string    `json:"issuer"`
			Subject     string    `json:"subject"`
			Role        string    `json:"role"`
		}{token.AccessToken, token.Expiry, p.Issuer, p.Subject, p.Role})
		if err != nil || save(h.file, payload) != nil {
			http.Error(w, "Private token file could not be saved", 500)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		h.success = true
		_, _ = w.Write([]byte("Login complete. The access token was saved privately. You may close this tab.\n"))
	default:
		http.NotFound(w, r)
	}
}

func query(r *http.Request) (map[string]string, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for key, v := range values {
		if len(v) != 1 || (key != "code" && key != "state" && key != "session_state" && key != "iss" && key != "error" && key != "error_description") {
			return nil, errors.New("invalid callback")
		}
		out[key] = v[0]
	}
	return out, nil
}

func save(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".token-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err != nil {
		return err
	}
	if cerr != nil {
		return cerr
	}
	return os.Rename(f.Name(), path)
}
