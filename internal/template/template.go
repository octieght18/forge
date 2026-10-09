// Package template writes the minimal service and MCP agent starting points.
// Both use the fixed research contract. Rendering does not register a workload,
// call the API, embed credentials, or install another orchestrator.
package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/octieght18/forge/internal/contract"
)

const (
	Service = "service"
	Agent   = "agent"
)

var ErrUnknownKind = errors.New("unknown template kind")
var ErrInvalidName = errors.New("invalid workload name")
var ErrExists = errors.New("template file already exists")

// Files returns the filenames a kind renders, in write order.
func Files(kind string) ([]string, error) {
	switch kind {
	case Service:
		return []string{"workload.json", "version.json", "health.json", "telemetry.json", "environment.json", "availability.json"}, nil
	case Agent:
		return []string{"workload.json", "version.json", "run.json", "health.json", "telemetry.json", "environment.json", "availability.json"}, nil
	default:
		return nil, ErrUnknownKind
	}
}

// Render builds the template files for one lowercase workload name.
func Render(validator *contract.Validator, kind, name string) ([]File, error) {
	names, err := Files(kind)
	if err != nil {
		return nil, err
	}
	description := "Registered research service using the fixed read-only contract."
	if kind == Agent {
		description = "Read-only MCP research agent using corpus.search and corpus.read."
	}
	workload, err := encode(map[string]string{"name": name, "description": description})
	if err != nil {
		return nil, err
	}
	if validator.ValidateRequest("CreateWorkloadRequest", workload) != nil {
		return nil, ErrInvalidName
	}
	version, err := example("examples/create-version.json")
	if err != nil {
		return nil, err
	}
	if validator.ValidateRequest("CreateVersionRequest", version) != nil {
		return nil, fmt.Errorf("embedded version template drifted from the contract")
	}
	bodies := map[string][]byte{
		"workload.json":     workload,
		"version.json":      version,
		"health.json":       mustEncode(healthDocument),
		"telemetry.json":    mustEncode(telemetryDocument),
		"environment.json":  mustEncode(environmentDocument),
		"availability.json": mustEncode(availabilityDocument),
	}
	if kind == Agent {
		run, err := example("examples/create-run.json")
		if err != nil {
			return nil, err
		}
		if validator.ValidateRequest("CreateRunRequest", run) != nil {
			return nil, fmt.Errorf("embedded run template drifted from the contract")
		}
		bodies["run.json"] = run
	}
	files := make([]File, 0, len(names))
	for _, filename := range names {
		files = append(files, File{Name: filename, Data: bodies[filename]})
	}
	return files, nil
}

type File struct {
	Name string
	Data []byte
}

// Write creates every file or leaves an existing directory unchanged when a target exists.
func Write(dir string, files []File) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, file := range files {
		if _, err := os.Lstat(filepath.Join(dir, file.Name)); err == nil {
			return fmt.Errorf("%w: %s", ErrExists, file.Name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, file := range files {
		path := filepath.Join(dir, file.Name)
		handle, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		_, writeErr := handle.Write(file.Data)
		closeErr := handle.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func example(name string) ([]byte, error) {
	body, err := contract.Examples.ReadFile(name)
	if err != nil {
		return nil, err
	}
	if !bytes.HasSuffix(body, []byte("\n")) {
		body = append(body, '\n')
	}
	return body, nil
}

func encode(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mustEncode(value any) []byte {
	body, err := encode(value)
	if err != nil {
		panic(err)
	}
	return body
}

var healthDocument = map[string]any{
	"liveness":       map[string]string{"method": "GET", "path": "/healthz"},
	"readiness":      map[string]string{"method": "GET", "path": "/readyz"},
	"workload_probe": "not_exposed",
}

var telemetryDocument = map[string]any{
	"correlation": []string{"request_id", "workload_id", "version_id"},
	"omit":        []string{"credentials", "raw_prompts", "corpus_text"},
	"collector":   "not_included",
}

var environmentDocument = map[string]any{
	"apiVersion": "platform.forge.local/v1alpha1",
	"kind":       "ForgeEnvironment",
	"metadata":   map[string]string{"name": "workload-<workloadID>"},
	"spec": map[string]any{
		"workloadID": "<workloadID>",
		"owner":      map[string]string{"issuer": "<issuer>", "subject": "<subject>"},
		"profile":    "small-v1",
	},
}

var availabilityDocument = map[string]string{
	"register":          "available",
	"deploy":            "available",
	"run":               "not_available",
	"environment_apply": "operator_only",
}
