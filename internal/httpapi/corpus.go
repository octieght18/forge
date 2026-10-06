package httpapi

import (
	"encoding/json"
	"errors"

	"github.com/octieght18/forge/internal/contract"
)

// CorpusPolicy is operator approval metadata, not a mounted/verified corpus.
type CorpusPolicy struct{ snapshots map[string]map[string]bool }

func LoadCorpusPolicy(body []byte) (*CorpusPolicy, error) {
	v, err := contract.New()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || len(body) > contract.MaxBodyBytes || !json.Valid(body) {
		return nil, errors.New("invalid corpus policy")
	}
	// Each entry reuses the trusted spec's ID validators. Strict JSON structure is
	// validated by the embedded policy schema, including duplicate-key rejection.
	if err := v.ValidateRequest("CorpusPolicy", body); err != nil {
		return nil, err
	}
	var entries map[string][]string
	_ = json.Unmarshal(body, &entries)
	policy := &CorpusPolicy{snapshots: make(map[string]map[string]bool)}
	for snapshot, docs := range entries {
		set := make(map[string]bool)
		for _, id := range docs {
			set[id] = true
		}
		policy.snapshots[snapshot] = set
	}
	return policy, nil
}
func (p *CorpusPolicy) approve(payload []byte) bool {
	var input struct {
		Spec struct {
			Snapshot    string `json:"corpus_snapshot_id"`
			Permissions struct {
				IDs []string `json:"document_ids"`
			} `json:"permissions"`
		} `json:"spec"`
	}
	if json.Unmarshal(payload, &input) != nil {
		return false
	}
	allowed := p.snapshots[input.Spec.Snapshot]
	if len(allowed) == 0 {
		return false
	}
	for _, id := range input.Spec.Permissions.IDs {
		if !allowed[id] {
			return false
		}
	}
	return true
}
