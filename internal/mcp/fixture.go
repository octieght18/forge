package mcp

import (
	"bufio"
	"encoding/json"
	"io"
)

const fixtureExcerpt = "PostgreSQL owns product records."

// Serve is a deterministic corpus tool. It answers corpus.search and corpus.read
// for the synthetic platform-brief passage and nothing else.
func Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	out := bufio.NewWriter(w)
	for scanner.Scan() {
		var request struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			continue
		}
		var result any
		switch request.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "forge-mcp-fixture", "version": "f23"}}
		case "tools/call":
			result = fixtureCall(request.Params.Name, request.Params.Arguments)
		default:
			result = map[string]any{"content": []any{map[string]string{"type": "text", "text": `{"passages":[]}`}}}
		}
		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
		if err != nil {
			return err
		}
		if _, err = out.Write(append(body, '\n')); err != nil {
			return err
		}
		if err = out.Flush(); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func fixtureCall(name string, args map[string]any) map[string]any {
	if args["path"] != nil || args["command"] != nil {
		return map[string]any{"isError": true, "content": []any{map[string]string{"type": "text", "text": "denied"}}}
	}
	passages := []map[string]string{}
	switch name {
	case "corpus.search":
		for _, id := range stringList(args["document_ids"]) {
			if id == "platform-brief" {
				passages = append(passages, map[string]string{"document_id": "platform-brief", "passage_id": "passage-1", "excerpt": fixtureExcerpt})
			}
		}
	case "corpus.read":
		if args["document_id"] == "platform-brief" && args["passage_id"] == "passage-1" {
			passages = append(passages, map[string]string{"document_id": "platform-brief", "passage_id": "passage-1", "excerpt": fixtureExcerpt})
		}
	}
	body, _ := json.Marshal(map[string]any{"passages": passages})
	return map[string]any{"content": []any{map[string]string{"type": "text", "text": string(body)}}}
}

func stringList(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, _ := item.(string)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}
