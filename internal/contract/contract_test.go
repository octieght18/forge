package contract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func validator(t *testing.T) *Validator {
	t.Helper()
	v, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func example(t *testing.T, name string) []byte {
	t.Helper()
	payload, err := Examples.ReadFile("examples/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func alter(t *testing.T, name string, change func(map[string]any)) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(example(t, name), &value); err != nil {
		t.Fatal(err)
	}
	change(value)
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestOpenAPIDocumentAndReferences(t *testing.T) {
	meta, err := os.ReadFile("testdata/openapi-meta.json")
	if err != nil {
		t.Fatal(err)
	}
	checksum := sha256.Sum256(meta)
	if hex.EncodeToString(checksum[:]) != "d0a3955182364c7b5fdebfd0583ecad259a870b4a2fe86a1b0fe8785f8224fed" {
		t.Fatal("official OpenAPI meta-schema changed without provenance update")
	}
	var metaValue, docValue any
	if err := json.Unmarshal(meta, &metaValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(OpenAPI(), &docValue); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(offlineLoader{})
	compiler.AssertFormat()
	const metaURL = "https://spec.openapis.org/oas/3.1/schema/2025-09-15"
	if err := compiler.AddResource(metaURL, metaValue); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(metaURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(docValue); err != nil {
		t.Fatal(err)
	}
	// Compile every embedded payload schema and its references as well. The OAS
	// structural meta-schema does not fully validate embedded JSON Schema contents.
	validator(t)
	doc := docValue.(map[string]any)
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if target, ok := value["$ref"].(string); ok {
				if !strings.HasPrefix(target, "#/") {
					t.Fatalf("non-local reference %s", target)
				}
				var resolved any = doc
				for _, part := range strings.Split(target[2:], "/") {
					object, ok := resolved.(map[string]any)
					if !ok {
						t.Fatalf("unresolved reference %s", target)
					}
					resolved, ok = object[part]
					if !ok {
						t.Fatalf("unresolved reference %s", target)
					}
				}
			}
			for _, child := range value {
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(doc)
	seen := map[string]bool{}
	for route, rawPath := range doc["paths"].(map[string]any) {
		for _, rawOp := range rawPath.(map[string]any) {
			op := rawOp.(map[string]any)
			id := op["operationId"].(string)
			if seen[id] {
				t.Fatalf("duplicate operationId %s", id)
			}
			seen[id] = true
			_, public := op["security"]
			if public && strings.HasPrefix(route, "/api/") {
				t.Fatalf("public product route %s", route)
			}
		}
	}
	copy := OpenAPI()
	copy[0] = 'x'
	if OpenAPI()[0] != '{' {
		t.Fatal("embedded document can be mutated by a caller")
	}
}

func TestPublishedExamples(t *testing.T) {
	v := validator(t)
	manifest, err := Examples.ReadFile("examples/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var examples map[string]string
	if err := json.Unmarshal(manifest, &examples); err != nil {
		t.Fatal(err)
	}
	for file, schema := range examples {
		t.Run(file, func(t *testing.T) {
			payload, err := Examples.ReadFile("examples/" + file)
			if err != nil {
				t.Fatal(err)
			}
			if err := v.Validate(schema, payload); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRejectUnsupportedRequests(t *testing.T) {
	v := validator(t)
	cases := []struct {
		name, schema, example string
		change                func(map[string]any)
	}{
		{"client-owner", "CreateWorkloadRequest", "create-workload", func(m map[string]any) { m["owner"] = map[string]any{"subject": "someone-else"} }},
		{"unknown-field", "CreateWorkloadRequest", "create-workload", func(m map[string]any) { m["desired_state"] = "running" }},
		{"empty-patch", "UpdateWorkloadRequest", "create-workload", func(m map[string]any) { delete(m, "name"); delete(m, "description") }},
		{"null-patch", "UpdateWorkloadRequest", "create-workload", func(m map[string]any) { m["description"] = nil }},
		{"empty-question", "CreateRunRequest", "create-run", func(m map[string]any) { m["question"] = " \n\t " }},
		{"unicode-blank-question", "CreateRunRequest", "create-run", func(m map[string]any) { m["question"] = "\u00a0\u2002\u3000" }},
		{"missing-version", "CreateRunRequest", "create-run", func(m map[string]any) { delete(m, "version_id") }},
		{"invalid-version-id", "CreateRunRequest", "create-run", func(m map[string]any) { m["version_id"] = "arbitrary" }},
		{"too-long-question", "CreateRunRequest", "create-run", func(m map[string]any) { m["question"] = strings.Repeat("a", 4001) }},
		{"path-source", "CreateRunRequest", "create-run", func(m map[string]any) { m["document_ids"] = []any{"../secrets"} }},
		{"duplicate-source", "CreateRunRequest", "create-run", func(m map[string]any) { m["document_ids"] = []any{"brief", "brief"} }},
		{"arbitrary-tool", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["tools"] = []any{"corpus.search", "shell.exec"} }},
		{"duplicate-tool", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["tools"] = []any{"corpus.read", "corpus.read"} }},
		{"missing-tool", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["tools"] = []any{"corpus.read"} }},
		{"write-permission", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["permissions"].(map[string]any)["write"] = true }},
		{"cpu-request", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["resources"] = map[string]any{"cpu": "1"} }},
		{"deployment-policy", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["deployment_policy"] = "canary" }},
		{"too-many-passages", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["limits"].(map[string]any)["max_passages"] = 17 }},
		{"fractional-limit", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["limits"].(map[string]any)["max_passages"] = 1.5 }},
		{"too-many-tokens", "CreateVersionRequest", "create-version", func(m map[string]any) {
			m["spec"].(map[string]any)["limits"].(map[string]any)["max_output_tokens"] = 2049
		}},
		{"paid-model", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["model_id"] = "some/paid-model" }},
		{"unknown-workflow", "CreateVersionRequest", "create-version", func(m map[string]any) { m["spec"].(map[string]any)["workflow_type"] = "arbitrary.v1" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := v.ValidateRequest(tc.schema, alter(t, tc.example, tc.change)); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected safe invalid-input error, got %v", err)
			}
		})
	}
}

func TestMalformedJSONAndSize(t *testing.T) {
	v := validator(t)
	for _, payload := range [][]byte{
		[]byte(`{"name":"a","name":"b","description":""}`),
		[]byte(`{"spec":{"tools":[],"tools":[]}}`),
		[]byte(`{} {}`), []byte(`{"name":`),
		[]byte(`{"name":"a","description":"\ud800"}`),
		[]byte(`{"name":"a","description":"\udc00"}`),
		[]byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'},
	} {
		if err := v.ValidateRequest("CreateWorkloadRequest", payload); !errors.Is(err, ErrInvalidJSON) {
			t.Fatalf("expected malformed JSON, got %v", err)
		}
	}
	if err := v.ValidateRequest("CreateRunRequest", bytes.Repeat([]byte(" "), MaxBodyBytes+1)); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected bounded body, got %v", err)
	}
}

func TestParserResourceBoundsAndUnicode(t *testing.T) {
	v := validator(t)
	for _, literal := range []string{"1e10000000", "1e-10000000", strings.Repeat("9", 129)} {
		payload := bytes.Replace(example(t, "create-version"), []byte(`"max_passages": 8`), []byte(`"max_passages": `+literal), 1)
		if err := v.ValidateRequest("CreateVersionRequest", payload); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("unbounded numeric spelling accepted: %v", err)
		}
	}
	deep := []byte(strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66))
	if err := v.Validate("CreateRunRequest", deep); !errors.Is(err, ErrInvalidJSON) {
		t.Fatal("unbounded nesting accepted")
	}
	for _, description := range []string{`\ud83d\ude00`, `\\ud800`, `valid UTF-8 😀`} {
		payload := []byte(`{"name":"valid","description":"` + description + `"}`)
		if err := v.ValidateRequest("CreateWorkloadRequest", payload); err != nil {
			t.Fatalf("valid Unicode rejected: %v", err)
		}
	}
}

func TestFingerprintEquivalenceAndConflict(t *testing.T) {
	v := validator(t)
	first, err := v.SubmissionFingerprint(example(t, "create-run"))
	if err != nil {
		t.Fatal(err)
	}
	reordered := alter(t, "create-run", func(m map[string]any) { m["document_ids"] = []any{"architecture-notes", "platform-brief"} })
	second, err := v.SubmissionFingerprint(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("unordered source selection changes fingerprint")
	}
	changed := alter(t, "create-run", func(m map[string]any) { m["question"] = "Different question" })
	third, err := v.SubmissionFingerprint(changed)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("changed question did not change fingerprint")
	}
	parent := "33333333-3333-4333-8333-333333333333"
	rerun, err := v.RerunFingerprint(parent)
	if err != nil {
		t.Fatal(err)
	}
	if rerun == first {
		t.Fatal("rerun shares createRun fingerprint")
	}
	version, err := v.VersionFingerprint(example(t, "create-version"))
	if err != nil {
		t.Fatal(err)
	}
	numeric := bytes.ReplaceAll(example(t, "create-version"), []byte(`"max_passages": 8`), []byte(`"max_passages": 8e0`))
	numeric = bytes.ReplaceAll(numeric, []byte(`"max_output_tokens": 1024`), []byte(`"max_output_tokens": 1024.0`))
	other, err := v.VersionFingerprint(numeric)
	if err != nil {
		t.Fatal(err)
	}
	if version != other {
		t.Fatal("equivalent JSON integer representations change spec fingerprint")
	}
}

func TestUncertaintyAndReportContract(t *testing.T) {
	v := validator(t)
	falseSuccess := alter(t, "unavailable-run", func(m map[string]any) { m["execution"].(map[string]any)["state"] = "succeeded" })
	if err := v.Validate("Run", falseSuccess); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("unavailable dependency can advertise current success")
	}
	invalidObservation := alter(t, "unavailable-run", func(m map[string]any) { m["execution"].(map[string]any)["observed_at"] = "yesterday" })
	if err := v.Validate("Run", invalidObservation); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("invalid observation timestamp accepted")
	}
	uncited := alter(t, "report", func(m map[string]any) { m["claims"].([]any)[0].(map[string]any)["evidence_ids"] = []any{} })
	if err := v.Validate("Report", uncited); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("uncited claim accepted")
	}
	blankClaim := alter(t, "report", func(m map[string]any) { m["claims"].([]any)[0].(map[string]any)["text"] = " \u00a0 " })
	if err := v.Validate("Report", blankClaim); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("blank claim accepted")
	}
	noClaims := alter(t, "report", func(m map[string]any) { m["claims"] = []any{} })
	if err := v.Validate("Report", noClaims); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("answered report without claims accepted")
	}
	noExplanation := alter(t, "report", func(m map[string]any) {
		m["outcome"] = "insufficient_evidence"
		m["claims"] = []any{}
		m["limitations"] = []any{}
	})
	if err := v.Validate("Report", noExplanation); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("insufficient evidence without limitations accepted")
	}
	for _, payload := range []string{`""`, `"bad key"`, `"key\nheader"`, `"` + strings.Repeat("k", 129) + `"`} {
		if err := v.Validate("IdempotencyKey", []byte(payload)); err == nil {
			t.Fatal("invalid idempotency key accepted")
		}
	}
	// A syntactically valid evidence ID still needs owner/run/snapshot checks at
	// the application boundary. Schema validation intentionally cannot grant access.
}
