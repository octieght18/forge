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
	"github.com/octieght18/forge/internal/provision"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestProvisioningOperations(t *testing.T) {
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
	tokens := map[string]string{}
	for _, role := range []string{"a", "b", "operator"} {
		mapped := "developer"
		if role == "operator" {
			mapped = role
		}
		tokens[role] = issuer.Token(t, role, mapped, nil)
	}
	call := func(method, path, user, body string) (int, map[string]any, http.Header) {
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
		response, err := server.Client().Do(req)
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
		return response.StatusCode, decoded, response.Header
	}
	status, created, _ := call("POST", "/api/v1/workloads", "a", `{"name":"async-boundary","description":"operation test"}`)
	if status != 201 {
		t.Fatal(status, created)
	}
	workloadID, _ := created["workload_id"].(string)
	example, err := contract.Examples.ReadFile("examples/create-version.json")
	if err != nil {
		t.Fatal(err)
	}
	status, versionBody, _ := call("POST", "/api/v1/workloads/"+workloadID+"/versions", "a", string(example))
	if status != 201 {
		t.Fatal(status, versionBody)
	}
	versionID, _ := versionBody["version_id"].(string)
	request := `{"action":"provision","version_id":"` + versionID + `","timeout_seconds":30}`
	status, accepted, header := call("POST", "/api/v1/workloads/"+workloadID+"/operations", "a", request)
	if status != 202 || accepted["status"] != "accepted" || header.Get("Location") == "" || accepted["status_url"] != header.Get("Location") {
		t.Fatal(status, accepted, header.Get("Location"))
	}
	operationID, _ := accepted["operation_id"].(string)
	if accepted["desired_generation"] != float64(1) || accepted["observed_generation"] != float64(0) {
		t.Fatal(accepted)
	}
	status, conflict, _ := call("POST", "/api/v1/workloads/"+workloadID+"/operations", "a", request)
	if status != 409 || conflict["error"].(map[string]any)["code"] != "conflict" {
		t.Fatal(status, conflict)
	}
	status, _, _ = call("POST", "/api/v1/workloads/"+workloadID+"/operations", "operator", request)
	if status != 403 {
		t.Fatal(status)
	}
	now := time.Now().UTC()
	if err := db.Repo.ObserveOperation(context.Background(), workloadID, provision.Observation{Phase: "Failed", Reason: "OwnershipConflict"}, now); err != nil {
		t.Fatal(err)
	}
	status, failed, _ := call("GET", "/api/v1/operations/"+operationID, "a", "")
	if status != 200 || failed["status"] != "failed" || failed["error"].(map[string]any)["code"] != "ownership_conflict" {
		t.Fatal(status, failed)
	}
	status, _, _ = call("GET", "/api/v1/operations/"+operationID, "b", "")
	if status != 404 {
		t.Fatal(status)
	}
	status, visible, _ := call("GET", "/api/v1/operations/"+operationID, "operator", "")
	if status != 200 || visible["status"] != "failed" {
		t.Fatal(status, visible)
	}
	status, finished, _ := call("POST", "/api/v1/operations/"+operationID+"/cancel", "a", `{}`)
	if status != 409 || finished["error"].(map[string]any)["message"] == "" {
		t.Fatal(status, finished)
	}
	stale := strings.Replace(request, versionID, "dddddddd-dddd-4ddd-8ddd-dddddddddddd", 1)
	status, _, _ = call("POST", "/api/v1/workloads/"+workloadID+"/operations", "a", stale)
	if status != 409 {
		t.Fatal(status)
	}
	status, again, header := call("POST", "/api/v1/workloads/"+workloadID+"/operations", "a", request)
	if status != 202 || header.Get("Location") == "" {
		t.Fatal(status, again)
	}
	second, _ := again["operation_id"].(string)
	status, canceling, _ := call("POST", "/api/v1/operations/"+second+"/cancel", "a", `{}`)
	if status != 202 || canceling["status"] != "cancel_requested" || canceling["action"] != "delete" || canceling["desired_generation"] != float64(3) {
		t.Fatal(status, canceling)
	}
	if err := db.Repo.ObserveOperation(context.Background(), workloadID, provision.Observation{Phase: "Absent", Reason: "CleanupComplete"}, now); err != nil {
		t.Fatal(err)
	}
	status, waiting, _ := call("GET", "/api/v1/operations/"+second, "a", "")
	if status != 200 || waiting["status"] != "cancel_requested" {
		t.Fatal(status, waiting)
	}
	if err := db.Repo.ObserveOperation(context.Background(), workloadID, provision.Observation{Phase: "Deleting", Reason: "CleanupPending"}, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Repo.ObserveOperation(context.Background(), workloadID, provision.Observation{Phase: "Absent", Reason: "CleanupComplete"}, now); err != nil {
		t.Fatal(err)
	}
	status, canceled, _ := call("GET", "/api/v1/operations/"+second, "a", "")
	if status != 200 || canceled["status"] != "canceled" || canceled["error"].(map[string]any)["code"] != "canceled" || canceled["observed_generation"] != canceled["desired_generation"] {
		t.Fatal(status, canceled)
	}
	status, third, _ := call("POST", "/api/v1/workloads/"+workloadID+"/operations", "a", request)
	if status != 202 {
		t.Fatal(status, third)
	}
	thirdID, _ := third["operation_id"].(string)
	if _, err := db.Migration.Exec(context.Background(), `UPDATE forge.provisioning_operations SET deadline = clock_timestamp() - interval '1 second' WHERE id=$1`, thirdID); err != nil {
		t.Fatal(err)
	}
	status, timedOut, _ := call("GET", "/api/v1/operations/"+thirdID, "a", "")
	if status != 200 || timedOut["status"] != "timed_out" || timedOut["error"].(map[string]any)["code"] != "operation_timeout" {
		t.Fatal(status, timedOut)
	}
	status, _, allow := call("DELETE", "/api/v1/operations/"+thirdID, "a", "")
	if status != 405 || !strings.Contains(allow.Get("Allow"), "GET") {
		t.Fatal(status, allow.Get("Allow"))
	}
	status, bad, _ := call("POST", "/api/v1/workloads/"+workloadID+"/operations", "a", `{"action":"provision","version_id":"`+versionID+`","timeout_seconds":0}`)
	if status != 422 || bad["error"].(map[string]any)["code"] != "invalid_input" {
		t.Fatal(status, bad)
	}
}
