// Package cli is the developer command line for the versioned registration API.
// register creates a workload, deploy records an immutable version, and status
// reads those records. delete calls the API and reports its result. This package
// does not provision environments, start runs, or remove product records itself.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/template"
)

const defaultAPI = "http://127.0.0.1:8081"

var (
	locationPattern = regexp.MustCompile(`^/api/v1/workloads/[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}(/versions/[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12})?$`)
	etagPattern     = regexp.MustCompile(`^"[0-9a-f-]{36}:[1-9][0-9]*"$`)
)

// Run executes one developer command. Successful JSON goes to stdout and
// failures go to stderr. Exit 0 is success, 1 is an API or transport failure,
// and 2 is a local validation or usage failure.
func Run(args []string, stdout, stderr io.Writer, client *http.Client) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usageText)
		return 0
	}
	command := args[0]
	switch command {
	case "register", "deploy", "status", "delete", "template":
	default:
		return fail(stderr, 2, command, 0, "", "invalid_input", "Unknown command. Use register, deploy, status, delete, or template.", "")
	}
	validator, err := contract.New()
	if err != nil {
		return fail(stderr, 1, command, 0, "", "unavailable", "Command could not load the API contract.", "")
	}
	app := &app{
		stdout: stdout, stderr: stderr, client: safeClient(client), validator: validator,
	}
	if command == "template" {
		return app.templates(args[1:])
	}
	return app.run(command, args[1:])
}

const usageText = `Forge developer CLI for the versioned registration API.

  forge register --token-file PATH --name NAME [--description TEXT] [--api URL]
  forge deploy --token-file PATH --workload ID --spec FILE [--api URL]
  forge status --token-file PATH [--workload ID] [--version ID] [--limit N] [--cursor TOKEN] [--api URL]
  forge delete --token-file PATH --workload ID [--api URL]
  forge template list
  forge template render --kind service|agent --name NAME --out DIRECTORY

register creates a workload. deploy records an immutable research version and does not start execution or provision an environment. status reads workloads and versions. delete calls the API; current servers reject workload and version deletion, and the JSON error includes that result. template writes a local service or MCP agent starting point and does not call the API. Success is one JSON document on stdout. Failures are one JSON document on stderr. request_id is the API operation ID when the API was reached.
`

type app struct {
	stdout, stderr io.Writer
	client         *http.Client
	validator      *contract.Validator
	api            string
	token          string
}

type requestRecord struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	RequestID string `json:"request_id,omitempty"`
}

type result struct {
	Operation         string          `json:"operation"`
	RequestID         string          `json:"request_id,omitempty"`
	Status            int             `json:"status"`
	Location          string          `json:"location,omitempty"`
	ETag              string          `json:"etag,omitempty"`
	Execution         string          `json:"execution,omitempty"`
	Environment       string          `json:"environment,omitempty"`
	Requests          []requestRecord `json:"requests"`
	Workload          json.RawMessage `json:"workload,omitempty"`
	Workloads         json.RawMessage `json:"workloads,omitempty"`
	Version           json.RawMessage `json:"version,omitempty"`
	Versions          json.RawMessage `json:"versions,omitempty"`
	VersionsRequestID string          `json:"versions_request_id,omitempty"`
}

func (a *app) run(command string, args []string) int {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tokenFile := fs.String("token-file", "", "private login JSON")
	api := fs.String("api", defaultAPI, "registration API base URL")
	name := fs.String("name", "", "workload name")
	description := fs.String("description", "", "workload description")
	workload := fs.String("workload", "", "workload ID")
	version := fs.String("version", "", "version ID")
	spec := fs.String("spec", "", "version JSON file")
	limit := fs.Int("limit", 20, "page size")
	cursor := fs.String("cursor", "", "opaque page cursor")
	if err := fs.Parse(args); err != nil {
		return fail(a.stderr, 2, command, 0, "", "invalid_input", clip(err.Error()), "")
	}
	if fs.NArg() != 0 {
		return fail(a.stderr, 2, command, 0, "", "invalid_input", "Unexpected command arguments.", "")
	}
	base, err := parseAPI(*api)
	if err != nil {
		return fail(a.stderr, 2, command, 0, "", "invalid_input", err.Error(), "")
	}
	token, err := readToken(*tokenFile)
	if err != nil {
		code := "invalid_input"
		if errors.Is(err, errExpired) {
			code = "unauthenticated"
		}
		return fail(a.stderr, 2, command, 0, "", code, err.Error(), "")
	}
	a.api = base
	a.token = token
	switch command {
	case "register":
		return a.register(*name, *description)
	case "deploy":
		return a.deploy(*workload, *spec)
	case "delete":
		return a.remove(*workload)
	default:
		return a.status(*workload, *version, *cursor, *limit, flagSet(fs, "limit"))
	}
}

func (a *app) templates(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(a.stdout, usageText)
		return 0
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fail(a.stderr, 2, "template", 0, "", "invalid_input", "template list takes no arguments.", "")
		}
		return writeTemplate(a.stdout, a.stderr, templateDocument{Action: "list", Templates: []templateInfo{
			{Kind: template.Service, Files: mustFiles(template.Service)},
			{Kind: template.Agent, Files: mustFiles(template.Agent)},
		}})
	case "render":
		return a.renderTemplate(args[1:])
	default:
		return fail(a.stderr, 2, "template", 0, "", "invalid_input", "Unknown template command. Use list or render.", "")
	}
}

func (a *app) renderTemplate(args []string) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	kind := fs.String("kind", "", "service or agent")
	name := fs.String("name", "", "workload name")
	out := fs.String("out", "", "output directory")
	if err := fs.Parse(args); err != nil {
		return fail(a.stderr, 2, "template", 0, "", "invalid_input", clip(err.Error()), "")
	}
	if fs.NArg() != 0 || *out == "" {
		return fail(a.stderr, 2, "template", 0, "", "invalid_input", "Render requires --kind, --name, and --out.", "")
	}
	files, err := template.Render(a.validator, *kind, *name)
	if err != nil {
		message := "Template could not be rendered."
		switch {
		case errors.Is(err, template.ErrUnknownKind):
			message = "Kind must be service or agent."
		case errors.Is(err, template.ErrInvalidName):
			message = "Name must be a lowercase slug of 1–63 characters."
		}
		return fail(a.stderr, 2, "template", 0, "", "invalid_input", message, "")
	}
	if err = template.Write(*out, files); err != nil {
		message := "Template directory could not be written."
		if errors.Is(err, template.ErrExists) {
			message = "A template file already exists in the output directory. Nothing was replaced."
		}
		return fail(a.stderr, 2, "template", 0, "", "invalid_input", message, "")
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Name)
	}
	return writeTemplate(a.stdout, a.stderr, templateDocument{Action: "render", Kind: *kind, Name: *name, Directory: *out, Files: names})
}

type templateInfo struct {
	Kind  string   `json:"kind"`
	Files []string `json:"files"`
}

type templateDocument struct {
	Operation string         `json:"operation"`
	Action    string         `json:"action"`
	Kind      string         `json:"kind,omitempty"`
	Name      string         `json:"name,omitempty"`
	Directory string         `json:"directory,omitempty"`
	Files     []string       `json:"files,omitempty"`
	Templates []templateInfo `json:"templates,omitempty"`
}

func writeTemplate(stdout, stderr io.Writer, doc templateDocument) int {
	doc.Operation = "template"
	if err := writeJSON(stdout, doc); err != nil {
		return fail(stderr, 1, "template", 0, "", "unavailable", "Could not write the command result.", "")
	}
	return 0
}

func mustFiles(kind string) []string {
	names, err := template.Files(kind)
	if err != nil {
		panic(err)
	}
	return names
}

func (a *app) register(name, description string) int {
	if strings.TrimSpace(name) == "" {
		return fail(a.stderr, 2, "register", 0, "", "invalid_input", "Name is required.", "")
	}
	body, err := json.Marshal(struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}{name, description})
	if err != nil {
		return fail(a.stderr, 2, "register", 0, "", "invalid_input", "Name and description could not be encoded.", "")
	}
	if err = a.validator.ValidateRequest("CreateWorkloadRequest", body); err != nil {
		return fail(a.stderr, 2, "register", 0, "", "invalid_input", schemaMessage(err, "Name and description", "Name and description do not match the supported schema. Names are lowercase slugs of 1–63 characters, and descriptions are at most 1,024 characters."), "")
	}
	call, err := a.call(http.MethodPost, "/api/v1/workloads", body)
	if err != nil {
		return a.transport("register", err)
	}
	if call.Status != http.StatusCreated {
		return a.apiError("register", call)
	}
	return a.ok(result{Operation: "register", RequestID: call.RequestID, Status: call.Status, Location: call.location, Requests: []requestRecord{call.record()}, Workload: call.body})
}

func (a *app) deploy(workload, specPath string) int {
	if err := a.validator.Validate("ID", []byte(strconv.Quote(workload))); err != nil {
		return fail(a.stderr, 2, "deploy", 0, "", "invalid_input", "Workload ID must be a lowercase UUID v4.", "")
	}
	if specPath == "" {
		return fail(a.stderr, 2, "deploy", 0, "", "invalid_input", "Spec file is required.", "")
	}
	body, err := readSpec(specPath)
	if err != nil {
		return fail(a.stderr, 2, "deploy", 0, "", "invalid_input", err.Error(), "")
	}
	if err = a.validator.ValidateRequest("CreateVersionRequest", body); err != nil {
		return fail(a.stderr, 2, "deploy", 0, "", "invalid_input", schemaMessage(err, "Spec", "Spec does not match the supported research version schema."), "")
	}
	call, err := a.call(http.MethodPost, "/api/v1/workloads/"+workload+"/versions", body)
	if err != nil {
		return a.transport("deploy", err)
	}
	if call.Status != http.StatusCreated {
		return a.apiError("deploy", call)
	}
	return a.ok(result{
		Operation: "deploy", RequestID: call.RequestID, Status: call.Status, Location: call.location,
		Execution: "not_started", Environment: "not_requested", Requests: []requestRecord{call.record()}, Version: call.body,
	})
}

func (a *app) remove(workload string) int {
	if err := a.validator.Validate("ID", []byte(strconv.Quote(workload))); err != nil {
		return fail(a.stderr, 2, "delete", 0, "", "invalid_input", "Workload ID must be a lowercase UUID v4.", "")
	}
	call, err := a.call(http.MethodDelete, "/api/v1/workloads/"+workload, nil)
	if err != nil {
		return a.transport("delete", err)
	}
	if call.Status < 200 || call.Status >= 300 {
		return a.apiError("delete", call)
	}
	return a.ok(result{Operation: "delete", RequestID: call.RequestID, Status: call.Status, Requests: []requestRecord{call.record()}, Workload: call.body})
}

func (a *app) status(workload, version, cursor string, limit int, limitSet bool) int {
	if version != "" && workload == "" {
		return fail(a.stderr, 2, "status", 0, "", "invalid_input", "A version ID requires a workload ID.", "")
	}
	if version != "" && (cursor != "" || limitSet) {
		return fail(a.stderr, 2, "status", 0, "", "invalid_input", "Limit and cursor apply to a collection, not one version.", "")
	}
	if workload != "" && a.validator.Validate("ID", []byte(strconv.Quote(workload))) != nil {
		return fail(a.stderr, 2, "status", 0, "", "invalid_input", "Workload ID must be a lowercase UUID v4.", "")
	}
	if version != "" && a.validator.Validate("ID", []byte(strconv.Quote(version))) != nil {
		return fail(a.stderr, 2, "status", 0, "", "invalid_input", "Version ID must be a lowercase UUID v4.", "")
	}
	if cursor != "" && a.validator.Validate("Cursor", []byte(strconv.Quote(cursor))) != nil {
		return fail(a.stderr, 2, "status", 0, "", "invalid_input", "Cursor must be the opaque value returned by the API.", "")
	}
	if limitSet && (limit < 1 || limit > 100) {
		return fail(a.stderr, 2, "status", 0, "", "invalid_input", "Limit must be an integer from 1 to 100.", "")
	}
	query := collectionQuery(limit, limitSet, cursor)
	if workload == "" {
		call, err := a.call(http.MethodGet, "/api/v1/workloads"+query, nil)
		if err != nil {
			return a.transport("status", err)
		}
		if call.Status != http.StatusOK {
			return a.apiError("status", call)
		}
		return a.ok(result{Operation: "status", RequestID: call.RequestID, Status: call.Status, Requests: []requestRecord{call.record()}, Workloads: call.body})
	}
	if version != "" {
		call, err := a.call(http.MethodGet, "/api/v1/workloads/"+workload+"/versions/"+version, nil)
		if err != nil {
			return a.transport("status", err)
		}
		if call.Status != http.StatusOK {
			return a.apiError("status", call)
		}
		return a.ok(result{Operation: "status", RequestID: call.RequestID, Status: call.Status, Requests: []requestRecord{call.record()}, Version: call.body})
	}
	parent, err := a.call(http.MethodGet, "/api/v1/workloads/"+workload, nil)
	if err != nil {
		return a.transport("status", err)
	}
	if parent.Status != http.StatusOK {
		return a.apiError("status", parent)
	}
	versions, err := a.call(http.MethodGet, "/api/v1/workloads/"+workload+"/versions"+query, nil)
	if err != nil {
		return a.transport("status", err)
	}
	if versions.Status != http.StatusOK {
		return a.apiError("status", versions)
	}
	return a.ok(result{
		Operation: "status", RequestID: parent.RequestID, Status: parent.Status, ETag: parent.etag,
		VersionsRequestID: versions.RequestID, Requests: []requestRecord{parent.record(), versions.record()},
		Workload: parent.body, Versions: versions.body,
	})
}

type exchange struct {
	Status    int
	RequestID string
	location  string
	etag      string
	body      json.RawMessage
	code      string
	message   string
	path      string
	method    string
}

func (e exchange) record() requestRecord {
	return requestRecord{Method: e.method, Path: e.path, Status: e.Status, RequestID: e.RequestID}
}

func (a *app) call(method, path string, body []byte) (exchange, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, a.api+path, reader)
	if err != nil {
		return exchange{}, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "forge-cli")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return exchange{}, err
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return exchange{}, err
	}
	if len(payload) > 1<<20 {
		return exchange{}, errors.New("response too large")
	}
	out := exchange{Status: resp.StatusCode, method: method, path: path, RequestID: safeToken(oneHeader(resp.Header, "X-Request-ID"), 128)}
	if loc := oneHeader(resp.Header, "Location"); locationPattern.MatchString(loc) {
		out.location = loc
	}
	if tag := oneHeader(resp.Header, "ETag"); etagPattern.MatchString(tag) {
		out.etag = tag
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if !json.Valid(payload) {
			return exchange{}, errors.New("invalid success JSON")
		}
		out.body = json.RawMessage(payload)
		return out, nil
	}
	var decoded struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if json.Unmarshal(payload, &decoded) == nil {
		out.code = safeText(decoded.Error.Code, a.token)
		out.message = safeText(decoded.Error.Message, a.token)
		if out.RequestID == "" {
			out.RequestID = safeToken(decoded.RequestID, 128)
		}
	}
	if out.code == "" || out.message == "" {
		if out.code == "" {
			out.code = "invalid_response"
		}
		if out.message == "" {
			out.message = "API returned an unreadable error."
		}
	}
	return out, nil
}

func (a *app) ok(value result) int {
	if err := writeJSON(a.stdout, value); err != nil {
		return fail(a.stderr, 1, value.Operation, 0, "", "unavailable", "Could not write the command result.", "")
	}
	return 0
}

func (a *app) apiError(operation string, call exchange) int {
	return fail(a.stderr, 1, operation, call.Status, call.RequestID, call.code, call.message, hint(operation, call.Status))
}

func (a *app) transport(operation string, err error) int {
	message := "The API could not be reached. Check the base URL and that the local stack is running."
	if errors.Is(err, errRedirect) {
		message = "The API redirected the request. The redirect was not followed and the access token was not sent onward."
	}
	return fail(a.stderr, 1, operation, 0, "", "unavailable", message, "")
}

func hint(operation string, status int) string {
	switch {
	case operation == "delete" && status == http.StatusMethodNotAllowed:
		return "The versioned API does not delete workloads or versions. Nothing was removed."
	case status == http.StatusUnauthorized:
		return "Log in again and pass the new private token file. Do not put the access token on the command line."
	case status == http.StatusNotFound:
		return "Check the ID and the identity that owns it. Another owner's object is also not found."
	case status == http.StatusConflict:
		return "This owner already has that workload name. Inspect the existing workload instead of registering it again."
	case status == http.StatusUnprocessableEntity:
		return "Use an approved research specification. Unsupported tools, models, deployment fields, and unknown properties are rejected."
	case status == http.StatusServiceUnavailable:
		return "Check API readiness. A timed-out register or deploy may already have been stored; inspect status before retrying."
	default:
		return ""
	}
}

type failureDoc struct {
	Operation string `json:"operation,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Status    int    `json:"status,omitempty"`
	Error     struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Hint string `json:"hint,omitempty"`
}

func fail(w io.Writer, code int, operation string, status int, requestID, errorCode, message, hintText string) int {
	doc := failureDoc{Operation: operation, RequestID: requestID, Status: status, Hint: hintText}
	doc.Error.Code = errorCode
	doc.Error.Message = message
	_ = writeJSON(w, doc)
	return code
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func schemaMessage(err error, subject, invalid string) string {
	switch {
	case errors.Is(err, contract.ErrBodyTooLarge):
		return subject + " exceeds 64 KiB."
	case errors.Is(err, contract.ErrInvalidJSON):
		return subject + " is not one UTF-8 JSON document."
	default:
		return invalid
	}
}

var errExpired = errors.New("access token is expired; log in again")
var errRedirect = errors.New("redirect")

func readToken(path string) (string, error) {
	if path == "" {
		return "", errors.New("Token file is required.")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("Token file could not be read.")
	}
	if len(payload) > 64*1024 {
		return "", errors.New("Token file is too large.")
	}
	var saved struct {
		AccessToken string    `json:"access_token"`
		ExpiresAt   time.Time `json:"expires_at"`
	}
	if json.Unmarshal(payload, &saved) != nil || saved.AccessToken == "" || saved.ExpiresAt.IsZero() {
		return "", errors.New("Token file must be the private login JSON, including access_token and expires_at.")
	}
	if !saved.ExpiresAt.After(time.Now()) {
		return "", errExpired
	}
	if !printableToken(saved.AccessToken) {
		return "", errors.New("Token file does not contain a usable access token.")
	}
	return saved.AccessToken, nil
}

func printableToken(token string) bool {
	if token == "" || len(token) > 16384 {
		return false
	}
	for i := 0; i < len(token); i++ {
		if token[i] <= ' ' || token[i] >= 0x7f {
			return false
		}
	}
	return true
}

func readSpec(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("Spec file could not be read.")
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, contract.MaxBodyBytes+1))
	if err != nil {
		return nil, errors.New("Spec file could not be read.")
	}
	return body, nil
}

func parseAPI(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || (parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New("API URL must be an absolute http or https URL with no credentials, query, or path.")
	}
	parsed.Path = ""
	return parsed.String(), nil
}

func collectionQuery(limit int, limitSet bool, cursor string) string {
	values := url.Values{}
	if limitSet {
		values.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		values.Set("cursor", cursor)
	}
	if encoded := values.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

func flagSet(fs *flag.FlagSet, name string) bool {
	seen := false
	fs.Visit(func(item *flag.Flag) {
		if item.Name == name {
			seen = true
		}
	})
	return seen
}

func oneHeader(header http.Header, key string) string {
	values := header.Values(key)
	if len(values) != 1 {
		return ""
	}
	return values[0]
}

func safeToken(value string, max int) string {
	if value == "" || len(value) > max {
		return ""
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '.' && c != '_' && c != ':' && c != '-' {
			return ""
		}
	}
	return value
}

func safeText(value, secret string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 300 || strings.ContainsAny(value, "\r\n") || len(secret) >= 16 && strings.Contains(value, secret) {
		return ""
	}
	return value
}

func clip(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 300 || strings.ContainsAny(value, "\r\n") {
		return "Invalid command arguments."
	}
	return value
}

func safeClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	copied := *client
	copied.CheckRedirect = func(*http.Request, []*http.Request) error { return errRedirect }
	return &copied
}
