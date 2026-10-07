package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/store"
)

type profileRecord struct {
	Kind           string `json:"kind"`
	Phase          string `json:"phase"`
	Client         int    `json:"client"`
	Journey        uint64 `json:"journey"`
	Operation      string `json:"operation,omitempty"`
	StartedNS      int64  `json:"started_unix_ns"`
	ElapsedNS      int64  `json:"elapsed_ns"`
	Status         int    `json:"status,omitempty"`
	TransportError bool   `json:"transport_error,omitempty"`
	Success        bool   `json:"success"`
}

type profilePhase struct {
	Phase          string    `json:"phase"`
	Started        time.Time `json:"started_utc"`
	Finished       time.Time `json:"finished_utc"`
	TargetSeconds  float64   `json:"target_seconds"`
	ElapsedSeconds float64   `json:"elapsed_seconds_including_drain"`
}

// Opt-in only: these are observations of the combined API/test-client process,
// not a benchmark of the resource-capped native deployment or real Keycloak.
func TestRegistrationLoadProfile(t *testing.T) {
	output := os.Getenv("FORGE_PROFILE_OUTPUT")
	if output == "" {
		t.Skip("set FORGE_PROFILE_OUTPUT to a new absolute directory for F10 profiles")
	}
	if !filepath.IsAbs(output) {
		t.Fatal("profile output must be absolute")
	}
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal("profile output must be a new directory", err)
	}
	warmup, duration := 5*time.Second, 15*time.Second
	if os.Getenv("FORGE_PROFILE_SMOKE") == "1" {
		warmup, duration = 100*time.Millisecond, 300*time.Millisecond
	}
	versionPayload, err := contract.Examples.ReadFile("examples/create-version.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, clients := range []int{1, 2} {
		t.Run(fmt.Sprintf("clients-%d", clients), func(t *testing.T) {
			f := newRegistrationFixture(t, 10*time.Second)
			// Private synthetic identities and disposable DB/roles come from the
			// shared fixture; no production DSN or bearer token is written out.
			dir := filepath.Join(output, fmt.Sprintf("clients-%d", clients))
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			raw, err := os.OpenFile(filepath.Join(dir, "requests.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			records := make(chan profileRecord, 64)
			written := make(chan error, 1)
			go func() {
				enc := json.NewEncoder(raw)
				var failure error
				for r := range records {
					if failure == nil {
						failure = enc.Encode(r)
					}
				}
				if err := raw.Close(); failure == nil {
					failure = err
				}
				written <- failure
			}()
			defer func() {
				close(records)
				if err := <-written; err != nil {
					t.Error(err)
				}
			}()
			var sequence atomic.Uint64
			journey := func(phase string, client int) bool {
				n := sequence.Add(1)
				started := time.Now()
				success := true
				request := func(operation, method, path, body string, want int) []byte {
					begin := time.Now()
					status, payload, err := f.request(context.Background(), method, path, body, "")
					ok := err == nil && status == want
					success = success && ok
					records <- profileRecord{Kind: "request", Phase: phase, Client: client, Journey: n, Operation: operation, StartedNS: begin.UnixNano(), ElapsedNS: time.Since(begin).Nanoseconds(), Status: status, TransportError: err != nil, Success: ok}
					return payload
				}
				body := request("create_workload", "POST", "/api/v1/workloads", fmt.Sprintf(`{"name":"profile-%d","description":"synthetic F10 fixture"}`, n), 201)
				var w store.Workload
				if success && (json.Unmarshal(body, &w) != nil || w.ID == "") {
					success = false
				}
				if success {
					path := "/api/v1/workloads/" + w.ID
					body = request("create_version", "POST", path+"/versions", string(versionPayload), 201)
					var v store.Version
					if success && (json.Unmarshal(body, &v) != nil || v.ID == "") {
						success = false
					}
					if success {
						request("read_workload", "GET", path, "", 200)
					}
					if success {
						request("read_version", "GET", path+"/versions/"+v.ID, "", 200)
					}
				}
				records <- profileRecord{Kind: "journey", Phase: phase, Client: client, Journey: n, StartedNS: started.UnixNano(), ElapsedNS: time.Since(started).Nanoseconds(), Success: success}
				return success
			}
			if !journey("preflight", 0) {
				t.Fatal("four-request preflight failed; inspect preserved raw output")
			}
			runPhase := func(phase string, length time.Duration) profilePhase {
				start := time.Now()
				deadline := start.Add(length)
				var wg sync.WaitGroup
				for client := 1; client <= clients; client++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						pprof.Do(context.Background(), pprof.Labels("component", "client"), func(context.Context) {
							for time.Now().Before(deadline) {
								journey(phase, client)
							}
						})
					}()
				}
				wg.Wait()
				finish := time.Now()
				return profilePhase{phase, start.UTC(), finish.UTC(), length.Seconds(), finish.Sub(start).Seconds()}
			}
			warm := runPhase("warmup", warmup)
			writeProfile := func(name string) {
				file, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				err = pprof.WriteHeapProfile(file)
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("heap write: %v %v", err, closeErr)
				}
			}
			runtime.GC()
			writeProfile("heap-before.pprof")
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			cpu, err := os.OpenFile(filepath.Join(dir, "cpu.pprof"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			if err := pprof.StartCPUProfile(cpu); err != nil {
				cpu.Close()
				t.Fatal(err)
			}
			measured := runPhase("measure", duration)
			pprof.StopCPUProfile()
			if err := cpu.Close(); err != nil {
				t.Fatal(err)
			}
			runtime.GC()
			runtime.ReadMemStats(&after)
			writeProfile("heap-after.pprof")
			var workloads, versions int
			if err := f.db.Pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM forge.workloads),(SELECT count(*) FROM forge.versions)`).Scan(&workloads, &versions); err != nil {
				t.Fatal(err)
			}
			metadata := map[string]any{"clients": clients, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "gomaxprocs": runtime.GOMAXPROCS(0), "pool_max_connections": f.db.Pool.Config().MaxConns, "operation_timeout_seconds": 5, "client_timeout_seconds": 10, "smoke": os.Getenv("FORGE_PROFILE_SMOKE") == "1", "phases": []profilePhase{warm, measured}, "heap_alloc_before_bytes": before.HeapAlloc, "heap_alloc_after_bytes": after.HeapAlloc, "total_alloc_delta_bytes": after.TotalAlloc - before.TotalAlloc, "gc_cycles_delta": after.NumGC - before.NumGC, "profile_scope": "API, closed-loop client, signed OIDC fixture, JSONL recorder and Go runtime in one process; PostgreSQL excluded"}
			metadata["source_commit"] = os.Getenv("FORGE_PROFILE_COMMIT")
			metadata["stored_workloads"], metadata["stored_versions"] = workloads, versions
			meta, err := json.MarshalIndent(metadata, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "conditions.json"), append(meta, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}
