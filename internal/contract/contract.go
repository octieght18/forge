// Package contract embeds the authoritative OpenAPI document and validates its
// JSON Schema payloads offline. It does not authenticate callers or persist runs.
package contract

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const MaxBodyBytes = 64 * 1024
const documentURL = "https://forge.invalid/openapi.json"

var (
	ErrInvalidJSON  = errors.New("invalid JSON document")
	ErrInvalidInput = errors.New("payload does not match the contract")
	ErrBodyTooLarge = errors.New("request body exceeds 64 KiB")
)

//go:embed openapi.json
var document []byte

//go:embed examples/*.json
var Examples embed.FS

// OpenAPI returns a copy so callers cannot mutate the embedded contract.
func OpenAPI() []byte { return bytes.Clone(document) }

type Validator struct{ schemas map[string]*jsonschema.Schema }

type offlineLoader struct{}

func (offlineLoader) Load(string) (any, error) {
	return nil, errors.New("external schema loading is disabled")
}

// New compiles trusted embedded schemas once. JSON Schema 2020-12 formats are
// assertions, including UUID and timestamp syntax. No network loader is used.
func New() (*Validator, error) {
	var doc map[string]any
	if err := json.Unmarshal(document, &doc); err != nil {
		return nil, fmt.Errorf("read OpenAPI: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	compiler.UseLoader(offlineLoader{})
	if err := compiler.AddResource(documentURL, doc); err != nil {
		return nil, fmt.Errorf("load schemas: %w", err)
	}
	components, ok := doc["components"].(map[string]any)
	if !ok {
		return nil, errors.New("missing OpenAPI components")
	}
	definitions, ok := components["schemas"].(map[string]any)
	if !ok {
		return nil, errors.New("missing OpenAPI schemas")
	}
	v := &Validator{schemas: make(map[string]*jsonschema.Schema, len(definitions))}
	for name := range definitions {
		schema, err := compiler.Compile(documentURL + "#/components/schemas/" + name)
		if err != nil {
			return nil, fmt.Errorf("compile %s: %w", name, err)
		}
		v.schemas[name] = schema
	}
	// Operator-only approval metadata; not another public product schema.
	policyURL := documentURL + "/corpus-policy"
	policy := map[string]any{"type": "object", "minProperties": 1, "maxProperties": 64,
		"propertyNames": map[string]any{"$ref": documentURL + "#/components/schemas/SHA256"},
		"additionalProperties": map[string]any{"type": "array", "minItems": 1, "maxItems": 64, "uniqueItems": true,
			"items": map[string]any{"$ref": documentURL + "#/components/schemas/DocumentID"}}}
	if err := compiler.AddResource(policyURL, policy); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(policyURL)
	if err != nil {
		return nil, err
	}
	v.schemas["CorpusPolicy"] = compiled
	return v, nil
}

// Validate checks a complete JSON value, including duplicate-key/UTF-8 rejection.
// It intentionally returns safe sentinel errors rather than echoing input values.
// Size enforcement belongs in ValidateRequest because paginated responses can be larger.
func (v *Validator) Validate(schemaName string, payload []byte) error {
	_, err := v.validatedValue(schemaName, payload)
	return err
}

func (v *Validator) ValidateRequest(schemaName string, payload []byte) error {
	if len(payload) > MaxBodyBytes {
		return ErrBodyTooLarge
	}
	return v.Validate(schemaName, payload)
}

func (v *Validator) validatedValue(schemaName string, payload []byte) (any, error) {
	schema, ok := v.schemas[schemaName]
	if !ok {
		return nil, fmt.Errorf("unknown contract schema %q", schemaName)
	}
	value, err := parseJSON(payload)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			return nil, ErrInvalidInput
		}
		return nil, ErrInvalidJSON
	}
	if err := schema.Validate(value); err != nil {
		return nil, ErrInvalidInput
	}
	return value, nil
}

// parseJSON preserves numbers and rejects duplicate object keys recursively.
// It does not accept multiple concatenated JSON documents or invalid UTF-8.
func parseJSON(payload []byte) (any, error) {
	if !utf8.Valid(payload) || !validSurrogateEscapes(payload) {
		return nil, ErrInvalidJSON
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	value, err := readValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrInvalidJSON
	}
	return value, nil
}

func readValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, ErrInvalidJSON
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	// Prevent tiny numeric spellings with enormous exponents from making the
	// exact-rational schema validator allocate unbounded integers. Request limits
	// are small integers; these guards preserve ordinary equivalent spellings.
	if number, ok := token.(json.Number); ok {
		literal := string(number)
		if len(literal) > 128 {
			return nil, ErrInvalidInput
		}
		if index := strings.IndexAny(literal, "eE"); index >= 0 {
			exponent, err := strconv.ParseInt(literal[index+1:], 10, 32)
			if err != nil || exponent < -308 || exponent > 308 {
				return nil, ErrInvalidInput
			}
		}
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delimiter {
	case '{':
		value := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, ErrInvalidJSON
			}
			if _, exists := value[key]; exists {
				return nil, ErrInvalidJSON
			}
			item, err := readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			value[key] = item
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
			return nil, ErrInvalidJSON
		}
		return value, nil
	case '[':
		value := []any{}
		for decoder.More() {
			item, err := readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			value = append(value, item)
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
			return nil, ErrInvalidJSON
		}
		return value, nil
	default:
		return nil, ErrInvalidJSON
	}
}

// encoding/json replaces lone UTF-16 surrogate escapes; reject them before that
// lossy conversion can make distinct question strings share a fingerprint.
func validSurrogateEscapes(payload []byte) bool {
	inside := false
	for i := 0; i < len(payload); i++ {
		if payload[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || payload[i] != '\\' {
			continue
		}
		i++
		if i >= len(payload) {
			return false
		}
		if payload[i] != 'u' {
			continue
		}
		if i+4 >= len(payload) {
			return false
		}
		code, err := strconv.ParseUint(string(payload[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return false
		}
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if i+6 >= len(payload) || payload[i+1] != '\\' || payload[i+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(payload[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}

// VersionFingerprint hashes the fully explicit immutable spec, normalizing
// unordered tool/document sets and JSON integer spellings. It is a content digest,
// not a credential, permission grant, signature or a client-selected version ID.
func (v *Validator) VersionFingerprint(payload []byte) (string, error) {
	if len(payload) > MaxBodyBytes {
		return "", ErrBodyTooLarge
	}
	value, err := v.validatedValue("CreateVersionRequest", payload)
	if err != nil {
		return "", err
	}
	return digest(normalize(value.(map[string]any)["spec"])), nil
}

// SubmissionFingerprint is only a content comparison for a database idempotency
// record. The future caller must key that record by validated issuer/subject/key,
// enforce ownership before lookup and perform durable acceptance transactionally.
func (v *Validator) SubmissionFingerprint(payload []byte) (string, error) {
	if len(payload) > MaxBodyBytes {
		return "", ErrBodyTooLarge
	}
	value, err := v.validatedValue("CreateRunRequest", payload)
	if err != nil {
		return "", err
	}
	return digest(map[string]any{"operation": "createRun", "request": normalize(value)}), nil
}

// RerunFingerprint occupies the same owner/key scope but a different operation.
func (v *Validator) RerunFingerprint(parentID string) (string, error) {
	encoded, _ := json.Marshal(parentID)
	if err := v.Validate("ID", encoded); err != nil {
		return "", err
	}
	return digest(map[string]any{"operation": "rerun", "parent_run_id": parentID}), nil
}

func normalize(value any) any {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			value[key] = normalize(item)
			if key == "document_ids" || key == "tools" {
				items := value[key].([]any)
				sort.Slice(items, func(i, j int) bool { return items[i].(string) < items[j].(string) })
			}
		}
		return value
	case []any:
		for index, item := range value {
			value[index] = normalize(item)
		}
		return value
	case json.Number:
		// Validated request numbers are bounded integer research limits <= 2048.
		number, _ := strconv.ParseFloat(string(value), 64)
		return json.Number(strconv.FormatFloat(number, 'f', -1, 64))
	default:
		return value
	}
}

func digest(value any) string {
	canonical, _ := json.Marshal(value) // validated JSON only; map keys are sorted
	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:])
}
