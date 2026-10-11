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

	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/template"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestThreeTemplatesRequestProvision(t *testing.T) {
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
		Repository: db.Repo, Auth: identity, Pool: db.Pool, Policy: policy, CursorKey: bytes.Repeat([]byte{1}, 32), Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	token := issuer.Token(t, "developer", "developer", nil)
	validator, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{}
	var registerMS, deployMS, provisionMS []float64
	for _, kind := range []string{template.Service, template.Agent, template.Job} {
		files, err := template.Render(validator, kind, "f19-"+kind)
		if err != nil {
			t.Fatal(err)
		}
		bodies := map[string][]byte{}
		for _, file := range files {
			bodies[file.Name] = file.Data
		}
		status, created, elapsed := timedCall(t, server, token, http.MethodPost, "/api/v1/workloads", bodies["workload.json"])
		if status != http.StatusCreated {
			t.Fatal(kind, status, created)
		}
		registerMS = append(registerMS, elapsed)
		workloadID, _ := created["workload_id"].(string)
		if workloadID == "" {
			t.Fatal("missing workload")
		}
		if _, ok := seen[workloadID]; ok {
			t.Fatal("duplicate workload", workloadID)
		}
		seen[workloadID] = struct{}{}
		status, version, elapsed := timedCall(t, server, token, http.MethodPost, "/api/v1/workloads/"+workloadID+"/versions", bodies["version.json"])
		if status != http.StatusCreated {
			t.Fatal(kind, status, version)
		}
		deployMS = append(deployMS, elapsed)
		versionID, _ := version["version_id"].(string)
		request := []byte(`{"action":"provision","version_id":"` + versionID + `","timeout_seconds":30}`)
		status, operation, elapsed := timedCall(t, server, token, http.MethodPost, "/api/v1/workloads/"+workloadID+"/operations", request)
		if status != http.StatusAccepted || operation["status"] != "accepted" {
			t.Fatal(kind, status, operation)
		}
		provisionMS = append(provisionMS, elapsed)
		t.Logf("%s register %.3fms deploy %.3fms provision %.3fms workload %s", kind, registerMS[len(registerMS)-1], deployMS[len(deployMS)-1], elapsed, workloadID)
	}
	if len(seen) != 3 {
		t.Fatal(len(seen))
	}
	t.Logf("samples register %v deploy %v provision %v", registerMS, deployMS, provisionMS)
}

func timedCall(t *testing.T, server *httptest.Server, token, method, path string, body []byte) (int, map[string]any, float64) {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := server.Client().Do(req)
	elapsed := time.Since(started).Seconds() * 1000
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if json.Unmarshal(payload, &decoded) != nil {
		t.Fatalf("status %d body %s", response.StatusCode, payload)
	}
	if strings.Contains(string(payload), token) {
		t.Fatal("response contains token")
	}
	return response.StatusCode, decoded, elapsed
}
