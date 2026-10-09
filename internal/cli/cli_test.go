package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const secret = "test-access-token-value"

func TestRegisterDeployStatusAndDelete(t *testing.T) {
	var spec []byte
	workloadID := "11111111-1111-4111-8111-111111111111"
	versionID := "22222222-2222-4222-8222-222222222222"
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.RequestURI())
		if r.Header.Get("Authorization") != "Bearer "+secret {
			t.Errorf("missing bearer token")
		}
		w.Header().Set("X-Request-ID", "0123456789abcdef0123456789abcdef")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workloads":
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("content type %q", r.Header.Get("Content-Type"))
			}
			body, _ := io.ReadAll(r.Body)
			var got map[string]string
			if json.Unmarshal(body, &got) != nil || got["name"] != "research-agent" || got["description"] != "example" {
				t.Errorf("workload body %s", body)
			}
			w.Header().Set("Location", "/api/v1/workloads/"+workloadID)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"workload_id":"` + workloadID + `","name":"research-agent"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workloads/"+workloadID+"/versions":
			body, _ := io.ReadAll(r.Body)
			if string(body) != string(spec) {
				t.Errorf("spec was rewritten")
			}
			w.Header().Set("Location", "/api/v1/workloads/"+workloadID+"/versions/"+versionID)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"version_id":"` + versionID + `","workload_id":"` + workloadID + `"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workloads/"+workloadID+"/versions/"+versionID:
			w.Header().Set("ETag", `"`+versionID+`:1"`)
			_, _ = w.Write([]byte(`{"version_id":"` + versionID + `"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workloads/"+workloadID && r.URL.RawQuery == "":
			w.Header().Set("ETag", `"`+workloadID+`:2"`)
			_, _ = w.Write([]byte(`{"workload_id":"` + workloadID + `","revision":2}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workloads/"+workloadID+"/versions":
			if r.URL.Query().Get("limit") != "1" || r.URL.Query().Get("cursor") != "abc_DEF-123" {
				t.Errorf("query %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"items":[{"version_id":"` + versionID + `"}],"next_cursor":null}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/workloads/"+workloadID:
			body, _ := io.ReadAll(r.Body)
			if len(body) != 0 {
				t.Errorf("delete sent a body")
			}
			w.Header().Set("Allow", "GET, PATCH")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"error":{"code":"method_not_allowed","message":"Method not allowed"},"request_id":"0123456789abcdef0123456789abcdef"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.RequestURI())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	token := writeToken(t, secret, time.Now().Add(time.Minute))
	spec = []byte(`{"spec":{"workflow_type":"research.v1","corpus_snapshot_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","tools":["corpus.search","corpus.read"],"permissions":{"document_ids":["platform-brief","architecture-notes"]},"limits":{"max_passages":8,"max_output_tokens":1024},"prompt_version":"research-report.v1","model_id":"google/gemma-4-26b-a4b-it:free"}}`)
	specFile := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(specFile, spec, 0o600); err != nil {
		t.Fatal(err)
	}

	registered := runJSON(t, 0, "register", "--token-file", token, "--api", server.URL, "--name", "research-agent", "--description", "example")
	if registered["request_id"] != "0123456789abcdef0123456789abcdef" || registered["location"] != "/api/v1/workloads/"+workloadID {
		t.Fatalf("register result %#v", registered)
	}
	deployed := runJSON(t, 0, "deploy", "--token-file", token, "--api", server.URL, "--workload", workloadID, "--spec", specFile)
	if deployed["execution"] != "not_started" || deployed["environment"] != "not_requested" || deployed["location"] != "/api/v1/workloads/"+workloadID+"/versions/"+versionID {
		t.Fatalf("deploy result %#v", deployed)
	}
	one := runJSON(t, 0, "status", "--token-file", token, "--api", server.URL, "--workload", workloadID, "--version", versionID)
	if one["request_id"] != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("version status %#v", one)
	}
	page := runJSON(t, 0, "status", "--token-file", token, "--api", server.URL, "--workload", workloadID, "--limit", "1", "--cursor", "abc_DEF-123")
	if page["versions_request_id"] != "0123456789abcdef0123456789abcdef" || page["etag"] != `"`+workloadID+`:2"` {
		t.Fatalf("collection status %#v", page)
	}
	requests, _ := page["requests"].([]any)
	if len(requests) != 2 {
		t.Fatalf("requests %#v", page["requests"])
	}
	removed := runJSON(t, 1, "delete", "--token-file", token, "--api", server.URL, "--workload", workloadID)
	errObj, _ := removed["error"].(map[string]any)
	if removed["status"] != float64(405) || errObj["code"] != "method_not_allowed" || removed["hint"] == "" || removed["request_id"] == "" {
		t.Fatalf("delete result %#v", removed)
	}
	if !strings.Contains(strings.Join(calls, "\n"), "DELETE /api/v1/workloads/"+workloadID) {
		t.Fatalf("calls %v", calls)
	}
}

func TestLocalValidationDoesNotCallAPI(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer server.Close()
	token := writeToken(t, secret, time.Now().Add(time.Minute))
	expired := writeToken(t, secret, time.Now().Add(-time.Minute))
	cases := []struct {
		name string
		args []string
		code string
	}{
		{"bad name", []string{"register", "--token-file", token, "--api", server.URL, "--name", "Not Slug", "--description", "x"}, "invalid_input"},
		{"missing spec", []string{"deploy", "--token-file", token, "--api", server.URL, "--workload", "11111111-1111-4111-8111-111111111111"}, "invalid_input"},
		{"bad id", []string{"delete", "--token-file", token, "--api", server.URL, "--workload", "nope"}, "invalid_input"},
		{"expired", []string{"status", "--token-file", expired, "--api", server.URL}, "unauthenticated"},
		{"credentialed url", []string{"status", "--token-file", token, "--api", "http://user:pass@127.0.0.1:8081"}, "invalid_input"},
		{"version without workload", []string{"status", "--token-file", token, "--api", server.URL, "--version", "22222222-2222-4222-8222-222222222222"}, "invalid_input"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runJSON(t, 2, tc.args...)
			errObj, _ := got["error"].(map[string]any)
			if errObj["code"] != tc.code || errObj["message"] == "" || strings.Contains(mustJSON(got), secret) {
				t.Fatalf("%#v", got)
			}
		})
	}
	if called {
		t.Fatal("validation failure reached the API")
	}
}

func TestAPIFailureKeepsTokenPrivate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "0123456789abcdef0123456789abcdef")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"conflict","message":"Resource conflicts with existing state and ` + secret + `"},"request_id":"0123456789abcdef0123456789abcdef"}`))
	}))
	defer server.Close()
	token := writeToken(t, secret, time.Now().Add(time.Minute))
	conflict := runJSON(t, 1, "register", "--token-file", token, "--api", server.URL, "--name", "research-agent", "--description", "example")
	errObj, _ := conflict["error"].(map[string]any)
	if errObj["code"] != "conflict" || strings.Contains(mustJSON(conflict), secret) || conflict["hint"] == "" || conflict["request_id"] == "" {
		t.Fatalf("%#v", conflict)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.invalid/steal", http.StatusFound)
	}))
	defer server.Close()
	token := writeToken(t, secret, time.Now().Add(time.Minute))
	got := runJSON(t, 1, "status", "--token-file", token, "--api", server.URL)
	errObj, _ := got["error"].(map[string]any)
	if errObj["code"] != "unavailable" || strings.Contains(mustJSON(got), secret) || strings.Contains(mustJSON(got), "example.invalid") {
		t.Fatalf("%#v", got)
	}
}

func TestTemplateRenderDoesNotCallAPI(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer server.Close()
	out := filepath.Join(t.TempDir(), "agent")
	rendered := runJSON(t, 0, "template", "render", "--kind", "agent", "--name", "research-agent", "--out", out)
	if rendered["action"] != "render" || rendered["kind"] != "agent" {
		t.Fatalf("%#v", rendered)
	}
	if _, err := os.Stat(filepath.Join(out, "run.json")); err != nil {
		t.Fatal(err)
	}
	again := runJSON(t, 2, "template", "render", "--kind", "agent", "--name", "research-agent", "--out", out)
	errObj, _ := again["error"].(map[string]any)
	if errObj["code"] != "invalid_input" {
		t.Fatalf("%#v", again)
	}
	listed := runJSON(t, 0, "template", "list")
	templates, _ := listed["templates"].([]any)
	if listed["action"] != "list" || len(templates) != 2 || called {
		t.Fatalf("%#v called=%v", listed, called)
	}
}

func TestHelpAndUnknownCommand(t *testing.T) {
	var out, err strings.Builder
	if code := Run([]string{"help"}, &out, &err, nil); code != 0 || !strings.Contains(out.String(), "forge deploy") {
		t.Fatalf("help %d %q", code, out.String())
	}
	out.Reset()
	err.Reset()
	if code := Run([]string{"nope"}, &out, &err, nil); code != 2 || !strings.Contains(err.String(), "Unknown command") || out.Len() != 0 {
		t.Fatalf("unknown %d %q %q", code, out.String(), err.String())
	}
}

func TestInvalidSpecIsRejectedLocally(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer server.Close()
	token := writeToken(t, secret, time.Now().Add(time.Minute))
	spec := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(spec, []byte(`{"spec":{"workflow_type":"other"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := runJSON(t, 2, "deploy", "--token-file", token, "--api", server.URL, "--workload", "11111111-1111-4111-8111-111111111111", "--spec", spec)
	errObj, _ := got["error"].(map[string]any)
	if errObj["code"] != "invalid_input" || !strings.Contains(errObj["message"].(string), "schema") || called {
		t.Fatalf("%#v called=%v", got, called)
	}
}

func writeToken(t *testing.T, token string, expires time.Time) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "access.json")
	payload, err := json.Marshal(map[string]any{"access_token": token, "expires_at": expires})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runJSON(t *testing.T, want int, args ...string) map[string]any {
	t.Helper()
	var stdout, stderr strings.Builder
	code := Run(args, &stdout, &stderr, nil)
	if code != want {
		t.Fatalf("exit %d, want %d\nstdout %s\nstderr %s", code, want, stdout.String(), stderr.String())
	}
	raw := stdout.String()
	if want != 0 {
		if stdout.Len() != 0 {
			t.Fatalf("failure wrote stdout %s", stdout.String())
		}
		raw = stderr.String()
	} else if stderr.Len() != 0 {
		t.Fatalf("success wrote stderr %s", stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("output is not JSON: %s", raw)
	}
	if strings.Contains(raw, secret) {
		t.Fatal("output contained the access token")
	}
	return got
}

func mustJSON(value any) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}
