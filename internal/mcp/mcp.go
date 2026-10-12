// Package mcp resolves a frozen research context and calls one read-only tool.
// The tool command comes from the worker, not from the workload. Credentials are
// not passed to the tool and are removed from errors.
package mcp

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/octieght18/forge/internal/contract"
)

const RetentionWorkflowHistory = "workflow_history"

var (
	ErrDenied = errors.New("tool denied")
	secret    = regexp.MustCompile(`(?i)bearer\s+[a-z0-9._~+/-]+=*|postgres(?:ql)?://\S+|password=[^\s]+`)
)

// Input is the frozen run context. It is resolved before a tool process starts.
type Input struct {
	RunID, WorkflowID, CommandID string
	SnapshotID                   string
	Question                     string
	DocumentIDs                  []string
	MaxPassages                  int
}

// Passage is one retrieved excerpt and its provenance.
type Passage struct {
	DocumentID string `json:"document_id"`
	PassageID  string `json:"passage_id"`
	EvidenceID string `json:"evidence_id,omitempty"`
	TextSHA256 string `json:"text_sha256"`
	Excerpt    string `json:"excerpt"`
}

// Evidence is a stored passage plus the retention intent for that record.
type Evidence struct {
	Passage
	RunID, WorkflowID, SnapshotID, Retention string
}

// Outcome is the authorized tool result. A denial does not start the tool.
type Outcome struct {
	Passages []Passage
	Class    string
	Failed   bool
}

// Tool performs one authorized call.
type Tool interface {
	Call(context.Context, string, map[string]any) (json.RawMessage, error)
}

var (
	mu       sync.Mutex
	evidence []Evidence
)

// Reset clears stored evidence. Tests use it.
func Reset() {
	mu.Lock()
	evidence = nil
	mu.Unlock()
}

// EvidenceRecords returns a copy of the passages stored for this process.
func EvidenceRecords() []Evidence {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Evidence, len(evidence))
	copy(out, evidence)
	return out
}

// Compensate marks evidence for a stopped run so it is not a committed result.
// A repeated call leaves an already compensated record unchanged.
func Compensate(runID string) {
	mu.Lock()
	defer mu.Unlock()
	for i := range evidence {
		if evidence[i].RunID == runID {
			evidence[i].Retention = "compensated"
		}
	}
}

// Search validates the frozen context, then calls corpus.search.
func Search(ctx context.Context, in Input, tool Tool) (Outcome, error) {
	args, err := resolve(in)
	if err != nil {
		return Outcome{Failed: true, Class: "tool_denied"}, nil
	}
	if tool == nil {
		return Outcome{Failed: true, Class: "tool_unavailable"}, nil
	}
	body, err := tool.Call(ctx, "corpus.search", args)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Outcome{}, err
		}
		return Outcome{Failed: true, Class: "tool_unavailable"}, errors.New(Redact(err.Error()))
	}
	var parsed struct {
		Passages []Passage `json:"passages"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return Outcome{Failed: true, Class: "tool_invalid"}, nil
	}
	allowed := map[string]bool{}
	for _, id := range in.DocumentIDs {
		allowed[id] = true
	}
	var kept []Passage
	for _, passage := range parsed.Passages {
		if !allowed[passage.DocumentID] || passage.PassageID == "" || passage.Excerpt == "" {
			continue
		}
		sum := sha256.Sum256([]byte(passage.Excerpt))
		passage.TextSHA256 = hex.EncodeToString(sum[:])
		passage.EvidenceID = newEvidenceID()
		kept = append(kept, passage)
		mu.Lock()
		evidence = append(evidence, Evidence{Passage: passage, RunID: in.RunID, WorkflowID: in.WorkflowID, SnapshotID: in.SnapshotID, Retention: RetentionWorkflowHistory})
		mu.Unlock()
	}
	return Outcome{Passages: kept}, nil
}

func resolve(in Input) (map[string]any, error) {
	if in.Question == "" || strings.TrimSpace(in.Question) == "" || len(in.DocumentIDs) == 0 || len(in.SnapshotID) != 64 {
		return nil, ErrDenied
	}
	limit := in.MaxPassages
	if limit < 1 || limit > 16 {
		limit = 8
	}
	args := map[string]any{
		"tool": "corpus.search", "corpus_snapshot_id": in.SnapshotID,
		"document_ids": in.DocumentIDs, "query": in.Question, "limit": limit,
	}
	body, err := json.Marshal(args)
	if err != nil {
		return nil, ErrDenied
	}
	validator, err := contract.New()
	if err != nil {
		return nil, err
	}
	if validator.Validate("MCPInvocation", body) != nil {
		return nil, ErrDenied
	}
	return args, nil
}

// Redact removes credential-shaped text from a tool error.
func Redact(message string) string {
	return secret.ReplaceAllString(message, "[redacted]")
}

func newEvidenceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Stdio is the worker-owned MCP command. It does not inherit the worker environment.
type Stdio struct{ Command string }

// AllowedTool is the only pair a run may execute.
func AllowedTool(name string) bool {
	return name == "corpus.search" || name == "corpus.read"
}

func (s Stdio) Call(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	if !AllowedTool(name) || s.Command == "" || strings.ContainsAny(s.Command, " \t\r\n|&;<>") {
		return nil, ErrDenied
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, s.Command)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()
	return callRPC(stdin, stdout, name, args)
}

func callRPC(w io.Writer, raw io.Reader, name string, args map[string]any) (json.RawMessage, error) {
	reader := bufio.NewReader(raw)
	if err := writeRPC(w, 1, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "forge", "version": "f23"}}); err != nil {
		return nil, err
	}
	if _, err := readRPC(reader); err != nil {
		return nil, err
	}
	if err := writeRPC(w, 2, "tools/call", map[string]any{"name": name, "arguments": args}); err != nil {
		return nil, err
	}
	response, err := readRPC(reader)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, errors.New(Redact(response.Error.Message))
	}
	var result struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(response.Result, &result) != nil || result.IsError || len(result.Content) == 0 {
		return nil, errors.New("tool result was not usable")
	}
	return json.RawMessage(result.Content[0].Text), nil
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func writeRPC(w io.Writer, id int, method string, params any) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return err
	}
	_, err = w.Write(append(body, '\n'))
	return err
}

func readRPC(r *bufio.Reader) (rpcResponse, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return rpcResponse{}, err
	}
	var response rpcResponse
	if json.Unmarshal(line, &response) != nil {
		return rpcResponse{}, errors.New("tool response was not valid")
	}
	return response, nil
}
