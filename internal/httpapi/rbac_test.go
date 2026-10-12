package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/audit"
	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestRoleMatrixAndDeniedTools(t *testing.T) {
	db := testsupport.NewDatabase(t)
	issuer := testsupport.NewIdentity(t)
	identity, err := auth.New(context.Background(), issuer.Server.URL, "forge-api")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := LoadCorpusPolicy([]byte(`{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":["platform-brief","architecture-notes"]}`))
	if err != nil {
		t.Fatal(err)
	}
	ready := &Readiness{}
	ready.Set(true)
	handler, err := NewRegistrationHandler(slog.New(slog.NewJSONHandler(io.Discard, nil)), ready, Registration{
		Repository: db.Repo, Auth: identity, Pool: db.Pool, Policy: policy, CursorKey: bytes.Repeat([]byte{4}, 32), Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	developer := issuer.Token(t, "a", "developer", nil)
	operator := issuer.Token(t, "operator", "operator", nil)
	service := issuer.Token(t, "dispatcher", "service", map[string]any{"identity_kind": "service"})
	mixed := issuer.Token(t, "a", "developer", map[string]any{"resource_access": map[string]any{"forge-api": map[string]any{"roles": []string{"developer", "operator", "service"}}}})
	call := func(t *testing.T, method, path, token, body string, headers map[string]string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		payload, _ := io.ReadAll(response.Body)
		return response.StatusCode, payload
	}
	status, created := call(t, "POST", "/api/v1/workloads", developer, `{"name":"owned","description":"rbac"}`, nil)
	if status != 201 {
		t.Fatal(status, string(created))
	}
	var workload struct {
		ID string `json:"workload_id"`
	}
	if err := json.Unmarshal(created, &workload); err != nil || workload.ID == "" {
		t.Fatal(err, string(created))
	}
	status, _ = call(t, "PATCH", "/api/v1/workloads/"+workload.ID, operator, `{"description":"no"}`, map[string]string{"If-Match": `"x"`})
	if status != 404 {
		t.Fatal("operator mutation", status)
	}
	status, _ = call(t, "GET", "/api/v1/telemetry", developer, "", nil)
	if status != 403 {
		t.Fatal("developer telemetry", status)
	}
	status, body := call(t, "GET", "/api/v1/telemetry", operator, "", nil)
	if status != 200 || !strings.Contains(string(body), "{") {
		t.Fatal("operator telemetry", status, string(body))
	}
	status, _ = call(t, "POST", "/api/v1/workloads", service, `{"name":"svc","description":"no"}`, nil)
	if status != 403 {
		t.Fatal("service token", status)
	}
	status, _ = call(t, "POST", "/api/v1/workloads", mixed, `{"name":"mix","description":"no"}`, nil)
	if status != 403 {
		t.Fatal("mixed roles", status)
	}
	example, err := contract.Examples.ReadFile("examples/create-version.json")
	if err != nil {
		t.Fatal(err)
	}
	status, _ = call(t, "POST", "/api/v1/workloads/"+workload.ID+"/versions", developer, strings.Replace(string(example), "corpus.search", "corpus.write", 1), nil)
	if status != 422 {
		t.Fatal("denied tool version", status)
	}
	audit.Reset()
	status, _ = call(t, "GET", "/api/v1/workloads/"+workload.ID, developer, "", nil)
	if status != 200 {
		t.Fatal(status)
	}
	recorded := audit.Events()
	if len(recorded) != 1 || recorded[0].Actor != "a" || recorded[0].Tenant != store.DemoTenant || recorded[0].Operation != "GET" || recorded[0].Outcome != "success" || recorded[0].Target != "/api/v1/workloads/"+workload.ID {
		t.Fatalf("%#v", recorded)
	}
	encoded, _ := json.Marshal(recorded)
	if strings.Contains(string(encoded), "Bearer") || strings.Contains(string(encoded), developer) {
		t.Fatal(string(encoded))
	}
}
