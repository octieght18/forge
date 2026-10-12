package httpapi

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestCrossTenantEndpointsStayHidden(t *testing.T) {
	db := testsupport.NewDatabase(t)
	const (
		otherTenant  = "12121212-1212-4212-8212-121212121212"
		otherProject = "34343434-3434-4434-8434-343434343434"
	)
	if _, err := db.Migration.Exec(context.Background(), `INSERT INTO forge.tenants(id,name) VALUES ($1,'other')`, otherTenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migration.Exec(context.Background(), `INSERT INTO forge.projects(id,tenant_id,name) VALUES ($1,$2,'other')`, otherProject, otherTenant); err != nil {
		t.Fatal(err)
	}
	issuer := testsupport.NewIdentity(t)
	other := store.Principal{Issuer: issuer.Server.URL, Subject: "other-owner", Role: "developer", Tenant: otherTenant, Project: otherProject}
	workload, err := db.Repo.CreateWorkload(context.Background(), other, []byte(`{"name":"other-scope","description":"foreign"}`))
	if err != nil {
		t.Fatal(err)
	}
	versionPayload, err := contract.Examples.ReadFile("examples/create-version.json")
	if err != nil {
		t.Fatal(err)
	}
	version, err := db.Repo.CreateVersion(context.Background(), other, workload.ID, versionPayload)
	if err != nil {
		t.Fatal(err)
	}
	runPayload, err := contract.Examples.ReadFile("examples/create-run.json")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(string(runPayload), "11111111-1111-4111-8111-111111111111", workload.ID)
	body = strings.ReplaceAll(body, "22222222-2222-4222-8222-222222222222", version.ID)
	run, err := db.Repo.SubmitRun(context.Background(), other, "other-run", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	operation, err := db.Repo.AcceptOperation(context.Background(), other, workload.ID, version.ID, "provision", time.Minute, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
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
		Repository: db.Repo, Auth: identity, Pool: db.Pool, Policy: policy, CursorKey: bytes.Repeat([]byte{3}, 32), Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	tokens := map[string]string{
		"a":        issuer.Token(t, "a", "developer", nil),
		"operator": issuer.Token(t, "operator", "operator", nil),
	}
	paths := []struct{ method, path, payload string }{
		{"GET", "/api/v1/workloads/" + workload.ID, ""},
		{"PATCH", "/api/v1/workloads/" + workload.ID, `{"description":"no"}`},
		{"GET", "/api/v1/workloads/" + workload.ID + "/versions/" + version.ID, ""},
		{"POST", "/api/v1/workloads/" + workload.ID + "/versions", string(versionPayload)},
		{"POST", "/api/v1/workloads/" + workload.ID + "/operations", `{"action":"provision","version_id":"` + version.ID + `","timeout_seconds":30}`},
		{"GET", "/api/v1/operations/" + operation.ID, ""},
		{"POST", "/api/v1/runs", body},
		{"GET", "/api/v1/runs/" + run.ID, ""},
		{"POST", "/api/v1/runs/" + run.ID + "/cancel", ""},
		{"GET", "/api/v1/runs/" + run.ID + "/history", ""},
		{"GET", "/api/v1/runs/" + run.ID + "/result", ""},
		{"POST", "/api/v1/runs/" + run.ID + "/rerun", ""},
	}
	for _, user := range []string{"a", "operator"} {
		for _, path := range paths {
			req, err := http.NewRequest(path.method, server.URL+path.path, strings.NewReader(path.payload))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+tokens[user])
			if path.payload != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			if path.method == "POST" && strings.Contains(path.path, "/runs") {
				req.Header.Set("Idempotency-Key", "cross-"+user)
			}
			if path.method == "PATCH" {
				req.Header.Set("If-Match", `"missing"`)
			}
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := io.ReadAll(response.Body)
			response.Body.Close()
			want := 404
			if user == "operator" && path.method == "POST" && strings.HasSuffix(path.path, "/operations") {
				want = 403
			}
			if response.StatusCode != want {
				t.Fatalf("%s %s %s status %d body %s", user, path.method, path.path, response.StatusCode, raw)
			}
		}
	}
	req, err := http.NewRequest("GET", server.URL+"/api/v1/workloads", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tokens["operator"])
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 || strings.Contains(string(raw), "other-scope") {
		t.Fatalf("list leaked tenant data %d %s", response.StatusCode, raw)
	}
	var counted int
	if err := db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM forge.workloads WHERE tenant_id=$1`, store.DemoTenant).Scan(&counted); err != nil {
		t.Fatal(err)
	}
}
