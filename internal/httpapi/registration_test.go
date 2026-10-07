package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
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
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestRegistrationHTTPPostgreSQL(t *testing.T) {
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
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	options := Registration{Repository: db.Repo, Auth: identity, Pool: db.Pool, Policy: policy, CursorKey: bytes.Repeat([]byte{1}, 32), Timeout: 200 * time.Millisecond}
	handler, err := NewRegistrationHandler(logger, ready, options)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	tokens := map[string]string{}
	for _, role := range []string{"a", "b", "operator"} {
		mapped := "developer"
		if role == "operator" {
			mapped = role
		}
		tokens[role] = issuer.Token(t, role, mapped, nil)
	}
	validator, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	call := func(t *testing.T, method, path, user, body string, headers map[string]string) (int, []byte, http.Header) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if user != "" {
			req.Header.Set("Authorization", "Bearer "+tokens[user])
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		payload, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Header.Get("X-Request-ID")) != 32 || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("missing correlation/cache headers")
		}
		if response.StatusCode >= 400 && validator.Validate("ErrorResponse", payload) != nil {
			t.Fatal("invalid error envelope", string(payload))
		}
		codes := map[int]string{400: "invalid_request", 401: "unauthenticated", 403: "forbidden", 404: "not_found", 409: "conflict", 412: "precondition_failed", 413: "request_too_large", 415: "unsupported_media_type", 422: "invalid_input", 428: "precondition_required"}
		if expected := codes[response.StatusCode]; expected != "" {
			var envelope ErrorResponse
			json.Unmarshal(payload, &envelope)
			if envelope.Error.Code != expected {
				t.Fatalf("status %d code %s, expected %s", response.StatusCode, envelope.Error.Code, expected)
			}
		}
		return response.StatusCode, payload, response.Header
	}
	expect := func(t *testing.T, got, want int) {
		t.Helper()
		if got != want {
			t.Fatalf("got status %d, want %d", got, want)
		}
	}
	status, _, headers := call(t, "GET", "/api/v1/workloads", "", "", nil)
	expect(t, status, 401)
	if headers.Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("missing bearer challenge")
	}
	// F10: bounded input read precedes authentication; input validation does not.
	status, _, _ = call(t, "POST", "/api/v1/workloads", "", `not-json`, nil)
	expect(t, status, 401)
	status, _, _ = call(t, "POST", "/api/v1/workloads", "", strings.Repeat(" ", contract.MaxBodyBytes+1), nil)
	expect(t, status, 413)
	status, body, headers := call(t, "POST", "/api/v1/workloads", "a", `{"name":"first","description":"fixture"}`, nil)
	expect(t, status, 201)
	if err := validator.Validate("Workload", body); err != nil {
		t.Fatal(err)
	}
	var w store.Workload
	json.Unmarshal(body, &w)
	path := "/api/v1/workloads/" + w.ID
	tag := headers.Get("ETag")
	if headers.Get("Location") != path || tag != etag(w.ID, 1) {
		t.Fatal("incorrect location/ETag")
	}
	t.Run("owner and operator boundaries", func(t *testing.T) {
		status, _, _ := call(t, "GET", path, "b", "", nil)
		expect(t, status, 404)
		status, _, _ = call(t, "GET", path, "operator", "", nil)
		expect(t, status, 200)
		status, _, _ = call(t, "PATCH", path, "operator", `{"description":"forbidden"}`, nil)
		expect(t, status, 404)
		status, _, _ = call(t, "PATCH", path, "b", `{"description":"forbidden"}`, map[string]string{"If-Match": tag})
		expect(t, status, 404)
		status, body, _ := call(t, "GET", "/api/v1/workloads", "b", "", nil)
		expect(t, status, 200)
		var page struct {
			Items []store.Workload `json:"items"`
		}
		json.Unmarshal(body, &page)
		if len(page.Items) != 0 {
			t.Fatal("foreign workload leaked into list")
		}
		status, _, _ = call(t, "DELETE", path, "a", "", nil)
		expect(t, status, 405)
		status, _, _ = call(t, "POST", "/api/v1/runs", "a", `{}`, nil)
		expect(t, status, 404)
	})
	t.Run("validation and preconditions", func(t *testing.T) {
		cases := []struct {
			method, path, body string
			headers            map[string]string
			want               int
		}{
			{"POST", "/api/v1/workloads", `{"name":"first","description":""}`, nil, 409},
			{"POST", "/api/v1/workloads", `{"name":"second","description":"","owner":"b"}`, nil, 422},
			{"POST", "/api/v1/workloads", `{"name":"a","name":"b"}`, nil, 400},
			{"POST", "/api/v1/workloads", `{"name":"test"}`, map[string]string{"Content-Type": "text/plain"}, 415},
			{"POST", "/api/v1/workloads", strings.Repeat(" ", contract.MaxBodyBytes+1), nil, 413},
			{"PATCH", path, `{"description":"x"}`, nil, 428},
			{"PATCH", path, `{}`, map[string]string{"If-Match": tag}, 422},
			{"PATCH", path, `{"description":null}`, map[string]string{"If-Match": tag}, 422},
			{"PATCH", path, `{"description":"x"}`, map[string]string{"If-Match": "W/" + tag}, 400},
			{"PATCH", path, `{"description":"x"}`, map[string]string{"If-Match": etag(w.ID, 99)}, 412},
			{"GET", "/api/v1/workloads?limit=101", "", nil, 400},
			{"GET", "/api/v1/workloads?limit=1&limit=2", "", nil, 400},
			{"GET", "/api/v1/workloads?cursor=forged", "", nil, 400},
		}
		for _, tc := range cases {
			status, _, _ := call(t, tc.method, tc.path, "a", tc.body, tc.headers)
			expect(t, status, tc.want)
		}
		status, payload, h := call(t, "PATCH", path, "a", `{"description":"changed"}`, map[string]string{"If-Match": tag})
		expect(t, status, 200)
		if validator.Validate("Workload", payload) != nil || h.Get("ETag") != etag(w.ID, 2) {
			t.Fatal("patch contract")
		}
	})
	var version store.Version
	t.Run("approved immutable versions", func(t *testing.T) {
		payload, _ := contract.Examples.ReadFile("examples/create-version.json")
		status, body, _ := call(t, "POST", path+"/versions", "a", string(payload), nil)
		expect(t, status, 201)
		if validator.Validate("WorkloadVersion", body) != nil {
			t.Fatal("version contract")
		}
		json.Unmarshal(body, &version)
		status, _, _ = call(t, "POST", path+"/versions", "a", strings.Replace(string(payload), "platform-brief", "not-approved", 1), nil)
		expect(t, status, 422)
		status, _, _ = call(t, "GET", path+"/versions/"+version.ID, "b", "", nil)
		expect(t, status, 404)
		status, _, _ = call(t, "GET", path+"/versions/"+version.ID, "operator", "", nil)
		expect(t, status, 200)
		status, _, _ = call(t, "POST", path+"/versions", "operator", string(payload), nil)
		expect(t, status, 404)
		status, _, _ = call(t, "PATCH", path+"/versions/"+version.ID, "a", string(payload), nil)
		expect(t, status, 405)
		// F04 registration retries may create another immutable version.
		status, _, _ = call(t, "POST", path+"/versions", "a", string(payload), map[string]string{"Idempotency-Key": "registration-is-not-idempotent"})
		expect(t, status, 201)
		status, body, _ = call(t, "GET", path+"/versions?limit=1", "a", "", nil)
		expect(t, status, 200)
		if validator.Validate("VersionPage", body) != nil {
			t.Fatal("version page contract")
		}
		var page struct {
			Next *string `json:"next_cursor"`
		}
		json.Unmarshal(body, &page)
		if page.Next == nil {
			t.Fatal("missing version cursor")
		}
		status, _, _ = call(t, "GET", path+"/versions?limit=1&cursor="+*page.Next, "a", "", nil)
		expect(t, status, 200)
	})
	t.Run("caller bound cursor pages", func(t *testing.T) {
		for _, name := range []string{"page-b", "page-c"} {
			status, _, _ := call(t, "POST", "/api/v1/workloads", "a", `{"name":"`+name+`","description":""}`, nil)
			expect(t, status, 201)
		}
		status, body, _ := call(t, "GET", "/api/v1/workloads?limit=1", "a", "", nil)
		expect(t, status, 200)
		if validator.Validate("WorkloadPage", body) != nil {
			t.Fatal("workload page contract")
		}
		var page struct {
			Items []store.Workload `json:"items"`
			Next  *string          `json:"next_cursor"`
		}
		json.Unmarshal(body, &page)
		if page.Next == nil {
			t.Fatal("missing cursor")
		}
		status, _, _ = call(t, "GET", "/api/v1/workloads?limit=1&cursor="+*page.Next, "b", "", nil)
		expect(t, status, 400)
		status, _, _ = call(t, "GET", path+"/versions?cursor="+*page.Next, "a", "", nil)
		expect(t, status, 400)
		status, body, _ = call(t, "GET", "/api/v1/workloads?limit=1&cursor="+*page.Next, "a", "", nil)
		expect(t, status, 200)
		var second struct {
			Items []store.Workload `json:"items"`
		}
		json.Unmarshal(body, &second)
		if len(second.Items) != 1 || second.Items[0].ID == page.Items[0].ID {
			t.Fatal("unstable paging")
		}
	})
	t.Run("concurrent HTTP revision conflict", func(t *testing.T) {
		const n = 4
		var wg sync.WaitGroup
		results := make(chan int, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				status, _, _ := call(t, "PATCH", path, "a", `{"description":"race"}`, map[string]string{"If-Match": etag(w.ID, 2)})
				results <- status
			}()
		}
		wg.Wait()
		close(results)
		success := 0
		for status := range results {
			if status == 200 {
				success++
			} else {
				expect(t, status, 412)
			}
		}
		if success != 1 {
			t.Fatal("concurrent update lost protection")
		}
	})
	t.Run("database lock timeout and recovery", func(t *testing.T) {
		tx, err := db.Migration.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		if _, err := tx.Exec(context.Background(), `SELECT id FROM forge.workloads WHERE id=$1 FOR UPDATE`, w.ID); err != nil {
			t.Fatal(err)
		}
		status, _, _ := call(t, "PATCH", path, "a", `{"description":"blocked"}`, map[string]string{"If-Match": etag(w.ID, 3)})
		expect(t, status, 503)
		tx.Rollback(context.Background())
		status, _, _ = call(t, "GET", path, "a", "", nil)
		expect(t, status, 200)
	})
	t.Run("database outage affects readiness but not liveness", func(t *testing.T) {
		var runtime string
		if err := db.Pool.QueryRow(context.Background(), `SELECT current_user`).Scan(&runtime); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Admin.Exec(context.Background(), "ALTER ROLE "+pgx.Identifier{runtime}.Sanitize()+" NOLOGIN"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Admin.Exec(context.Background(), `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE usename=$1`, runtime); err != nil {
			t.Fatal(err)
		}
		status, _, _ := call(t, "GET", path, "a", "", nil)
		expect(t, status, 503)
		status, _, _ = call(t, "GET", "/readyz", "", "", nil)
		expect(t, status, 503)
		status, _, _ = call(t, "GET", "/healthz", "", "", nil)
		expect(t, status, 200)
	})
}

func TestCursorsPolicyAndETagGuards(t *testing.T) {
	now := time.Unix(1900000000, 0)
	codec := cursors{bytes.Repeat([]byte{1}, 32), func() time.Time { return now }}
	p := store.Principal{Issuer: "https://identity.example", Subject: "a", Role: "developer"}
	token := codec.encode(p, cursor{Collection: "workloads", ID: "11111111-1111-4111-8111-111111111111", At: now})
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(token)
	if v.Validate("Cursor", b) != nil {
		t.Fatal("cursor violates contract")
	}
	if _, err := codec.decode(p, "workloads", "", token); err != nil {
		t.Fatal(err)
	}
	if _, err := codec.decode(p, "versions", "parent", token); err == nil {
		t.Fatal("wrong binding accepted")
	}
	if _, err := codec.decode(p, "workloads", "", token[:len(token)-2]+"zz"); err == nil {
		t.Fatal("tampering accepted")
	}
	now = now.Add(15 * time.Minute)
	if _, err := codec.decode(p, "workloads", "", token); err == nil {
		t.Fatal("expired cursor accepted")
	}
	for _, body := range []string{`{}`, `{"bad":["doc"]}`, `{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":["../../path"]}`, `{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":["doc","doc"]}`} {
		if _, err := LoadCorpusPolicy([]byte(body)); err == nil {
			t.Fatal("invalid policy accepted")
		}
	}
	if _, err := parseETag("11111111-1111-4111-8111-111111111111", []string{`"2"`}); err == nil {
		t.Fatal("legacy short ETag accepted")
	}
}
