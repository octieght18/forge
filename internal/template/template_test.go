package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/octieght18/forge/internal/contract"
)

func TestServiceAndAgentTemplates(t *testing.T) {
	validator, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{Service, Agent, Job} {
		t.Run(kind, func(t *testing.T) {
			files, err := Render(validator, kind, kind+"-example")
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if err := Write(dir, files); err != nil {
				t.Fatal(err)
			}
			if err := Write(dir, files); !errors.Is(err, ErrExists) {
				t.Fatalf("overwrite err %v", err)
			}
			bodies := map[string][]byte{}
			for _, file := range files {
				body, err := os.ReadFile(filepath.Join(dir, file.Name))
				if err != nil || !bytes.Equal(body, file.Data) {
					t.Fatalf("wrote %s differently", file.Name)
				}
				bodies[file.Name] = body
				lower := strings.ToLower(string(body))
				for _, banned := range []string{"password", "secret", "api_key", "bearer", "private_key", "helm", "kustomize", "docker-compose", "nomad", "argocd"} {
					if strings.Contains(lower, banned) {
						t.Fatalf("%s contains %s", file.Name, banned)
					}
				}
			}
			if validator.ValidateRequest("CreateWorkloadRequest", bodies["workload.json"]) != nil || validator.ValidateRequest("CreateVersionRequest", bodies["version.json"]) != nil {
				t.Fatal("workload or version is not a valid registration document")
			}
			var spec struct {
				Spec struct {
					Workflow string   `json:"workflow_type"`
					Tools    []string `json:"tools"`
				} `json:"spec"`
			}
			if json.Unmarshal(bodies["version.json"], &spec) != nil || spec.Spec.Workflow != "research.v1" || strings.Join(spec.Spec.Tools, ",") != "corpus.search,corpus.read" {
				t.Fatalf("spec %#v", spec.Spec)
			}
			var env map[string]any
			if json.Unmarshal(bodies["environment.json"], &env) != nil || env["apiVersion"] != "platform.forge.local/v1alpha1" || env["kind"] != "ForgeEnvironment" {
				t.Fatalf("environment %#v", env)
			}
			manifest := env["spec"].(map[string]any)
			if manifest["profile"] != "small-v1" || manifest["workloadID"] != "<workloadID>" {
				t.Fatalf("environment spec %#v", manifest)
			}
			var health map[string]any
			if json.Unmarshal(bodies["health.json"], &health) != nil || health["workload_probe"] != "not_exposed" {
				t.Fatal("health convention missing")
			}
			var telemetry map[string]any
			if json.Unmarshal(bodies["telemetry.json"], &telemetry) != nil || telemetry["collector"] != "not_included" {
				t.Fatal("telemetry convention missing")
			}
			_, agentRun := bodies["run.json"]
			if kind == Agent && (!agentRun || validator.ValidateRequest("CreateRunRequest", bodies["run.json"]) != nil) {
				t.Fatal("agent run template is missing or invalid")
			}
			if kind != Agent && agentRun {
				t.Fatal("non-agent template included a run")
			}
			resources, jobResources := bodies["resources.json"]
			if kind == Job {
				if !jobResources {
					t.Fatal("job resources missing")
				}
				var recorded struct {
					Profile string            `json:"profile"`
					Applied string            `json:"applied_by"`
					Quota   map[string]string `json:"quota"`
				}
				if json.Unmarshal(resources, &recorded) != nil || recorded.Profile != "small-v1" || recorded.Applied != "operator" || recorded.Quota["pods"] != "2" || recorded.Quota["persistentvolumeclaims"] != "0" {
					t.Fatalf("resources %#v", recorded)
				}
			}
			if kind != Job && jobResources {
				t.Fatal("resources included on a non-job template")
			}
		})
	}
}

func TestRejectsUnknownKindAndName(t *testing.T) {
	validator, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Render(validator, "platform", "research-agent"); !errors.Is(err, ErrUnknownKind) {
		t.Fatal(err)
	}
	if _, err := Render(validator, Service, "Not A Name"); !errors.Is(err, ErrInvalidName) {
		t.Fatal(err)
	}
}
