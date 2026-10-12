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
	"sync"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestSingleProcessQuota(t *testing.T) {
	db := testsupport.NewDatabase(t)
	db.Repo.SetLimits(2, 1)
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
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	newServer := func(perSecond int) *httptest.Server {
		t.Helper()
		handler, err := NewRegistrationHandler(logger, ready, Registration{
			Repository: db.Repo, Auth: identity, Pool: db.Pool, Policy: policy, CursorKey: bytes.Repeat([]byte{5}, 32), Timeout: time.Second,
			RequestsPerSecond: perSecond,
		})
		if err != nil {
			t.Fatal(err)
		}
		return httptest.NewServer(handler)
	}
	token := issuer.Token(t, "a", "developer", nil)
	call := func(server *httptest.Server, method, path, body string, headers map[string]string) (int, http.Header, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
		return response.StatusCode, response.Header, payload
	}
	limited := newServer(2)
	defer limited.Close()
	sawLimit := false
	for i := 0; i < 3; i++ {
		status, header, payload := call(limited, "GET", "/api/v1/workloads", "", nil)
		if status != 429 {
			continue
		}
		sawLimit = true
		if header.Get("Retry-After") != "1" || strings.Contains(string(payload), "goroutine") || !strings.Contains(string(payload), `"code":"rate_limited"`) {
			t.Fatal(header.Get("Retry-After"), string(payload))
		}
	}
	if !sawLimit {
		t.Fatal("rate limit did not trip")
	}
	fast := newServer(1000)
	defer fast.Close()
	var created, rejected int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "quota-" + string(rune('a'+i))
			status, _, payload := call(fast, "POST", "/api/v1/workloads", `{"name":"`+name+`","description":"race"}`, nil)
			mu.Lock()
			defer mu.Unlock()
			switch status {
			case 201:
				created++
			case 429:
				rejected++
				if strings.Contains(string(payload), "goroutine") {
					t.Errorf("stack in quota response %s", payload)
				}
			default:
				t.Errorf("status %d %s", status, payload)
			}
		}(i)
	}
	wg.Wait()
	if created != 2 || rejected != 6 {
		t.Fatalf("created %d rejected %d", created, rejected)
	}
	var stored int
	if err := db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM forge.workloads`).Scan(&stored); err != nil || stored != 2 {
		t.Fatal(err, stored)
	}
	status, _, workloadBody := call(fast, "GET", "/api/v1/workloads?limit=1", "", nil)
	if status != 200 {
		t.Fatal(status, string(workloadBody))
	}
	var page struct {
		Items []struct {
			ID string `json:"workload_id"`
		} `json:"items"`
	}
	if json.Unmarshal(workloadBody, &page) != nil || len(page.Items) != 1 {
		t.Fatal(string(workloadBody))
	}
	example, err := contract.Examples.ReadFile("examples/create-version.json")
	if err != nil {
		t.Fatal(err)
	}
	status, _, versionBody := call(fast, "POST", "/api/v1/workloads/"+page.Items[0].ID+"/versions", string(example), nil)
	if status != 201 {
		t.Fatal(status, string(versionBody))
	}
	var version struct {
		ID string `json:"version_id"`
	}
	json.Unmarshal(versionBody, &version)
	runBody := `{"workload_id":"` + page.Items[0].ID + `","version_id":"` + version.ID + `","question":"Which service owns execution?","document_ids":["platform-brief"]}`
	status, _, _ = call(fast, "POST", "/api/v1/runs", runBody, map[string]string{"Idempotency-Key": "quota-run-1"})
	if status != 202 {
		t.Fatal(status)
	}
	status, header, limitedBody := call(fast, "POST", "/api/v1/runs", runBody, map[string]string{"Idempotency-Key": "quota-run-2"})
	if status != 429 || header.Get("Retry-After") != "1" || strings.Contains(string(limitedBody), "goroutine") {
		t.Fatal(status, header.Get("Retry-After"), string(limitedBody))
	}
}
