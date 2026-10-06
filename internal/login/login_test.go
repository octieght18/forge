package login

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/octieght18/forge/internal/testsupport"
)

func TestBrowserPKCEAndPrivateToken(t *testing.T) {
	for _, role := range []string{"developer", "unapproved"} {
		t.Run(role, func(t *testing.T) {
			identity := testsupport.NewIdentity(t)
			file := filepath.Join(privateDir(t), "access.json")
			h, err := New(context.Background(), identity.Server.URL, "http://127.0.0.1:8083/callback", file)
			if err != nil {
				t.Fatal(err)
			}
			exchanges := 0
			tokenEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				exchanges++
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
				if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge(h.verifier) || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("client_id") != "forge-local-login" || r.Form.Get("redirect_uri") != h.config.RedirectURL || r.Form.Get("client_secret") != "" || r.Form.Get("code") != "accepted-code" {
					t.Error("Invalid public-client PKCE exchange")
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": identity.Token(t, "owner", role, nil), "token_type": "Bearer", "expires_in": 300})
			}))
			defer tokenEndpoint.Close()
			h.config.Endpoint.TokenURL = tokenEndpoint.URL
			call := func(path string, cookie *http.Cookie) *httptest.ResponseRecorder {
				r := httptest.NewRequest("GET", "http://127.0.0.1:8083"+path, nil)
				if cookie != nil {
					r.AddCookie(cookie)
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				return w
			}
			start := call("/login", nil)
			location, _ := url.Parse(start.Header().Get("Location"))
			if start.Code != 302 || location.Query().Get("code_challenge_method") != "S256" || location.Query().Get("code_challenge") != challenge(h.verifier) || location.Query().Get("state") == "" {
				t.Fatal("PKCE authorization missing")
			}
			cookie := start.Result().Cookies()[0]
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatal("Cookie policy")
			}
			for _, path := range []string{"/callback?state=wrong&code=x", "/callback?state=" + h.state + "&state=duplicate&code=x", "/callback?state=" + h.state + "&code=x&iss=http://evil.invalid"} {
				if call(path, cookie).Code != 400 || exchanges != 0 {
					t.Fatal("Unbound callback reached exchange")
				}
			}
			if call("/callback?state="+h.state+"&code=x", nil).Code != 400 {
				t.Fatal("Missing cookie accepted")
			}
			if call("/login", nil).Code != 409 {
				t.Fatal("Concurrent login replaced state")
			}
			finish := call("/callback?state="+h.state+"&code=accepted-code", cookie)
			if role == "developer" {
				if finish.Code != 200 || !h.Succeeded() {
					t.Fatalf("Login failed: %d", finish.Code)
				}
				var saved map[string]any
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if json.Unmarshal(data, &saved) != nil || saved["role"] != "developer" || saved["subject"] != "owner" || saved["access_token"] == "" {
					t.Fatal("Invalid saved access token")
				}
				info, _ := os.Stat(file)
				if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
					t.Fatal("Token file is not private")
				}
			} else if finish.Code != 403 || h.Succeeded() {
				t.Fatal("Unapproved token role accepted")
			}
			if call("/callback?state="+h.state+"&code=accepted-code", cookie).Code != 410 || exchanges != 1 {
				t.Fatal("Callback replayed")
			}
			select {
			case <-h.Done():
			default:
				t.Fatal("Completion missing")
			}
		})
	}
}

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestLoginHostAndCallbackValidation(t *testing.T) {
	i := testsupport.NewIdentity(t)
	file := filepath.Join(privateDir(t), "token.json")
	for _, callback := range []string{"http://evil.invalid/callback", "http://127.0.0.1:0/callback", "http://127.0.0.1:8083/callback?x=1", "http://127.0.0.1:8083@evil.invalid/callback"} {
		if _, err := New(context.Background(), i.Server.URL, callback, file); err == nil {
			t.Fatal("Unsafe callback accepted")
		}
	}
	h, err := New(context.Background(), i.Server.URL, "http://127.0.0.1:8083/callback", file)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://evil.invalid/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("Untrusted Host accepted")
	}
	r = httptest.NewRequest("POST", "http://127.0.0.1:8083/login", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatal("Unexpected method accepted")
	}
}

func privateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPrivateTokenFileReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.json")
	for _, value := range []string{"first", "replacement"} {
		if err := save(path, []byte(value)); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != value {
			t.Fatal("Token replacement failed")
		}
	}
}
