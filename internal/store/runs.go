package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/octieght18/forge/internal/contract"
)

type runInput struct {
	WorkloadID  string   `json:"workload_id"`
	VersionID   string   `json:"version_id"`
	Question    string   `json:"question"`
	DocumentIDs []string `json:"document_ids"`
}

const runColumns = `r.id::text,p.issuer,p.subject,r.workload_id::text,r.version_id::text,r.input,r.rerun_of::text,r.workflow_id,r.created_at`
const runJoin = ` FROM forge.runs r JOIN forge.principals p ON p.id=r.owner_id `

func scanRun(row pgx.Row) (Run, error) {
	var run Run
	err := row.Scan(&run.ID, &run.Owner.Issuer, &run.Owner.Subject, &run.WorkloadID, &run.VersionID, &run.Input, &run.RerunOf, &run.WorkflowID, &run.CreatedAt)
	return run, storageError(err)
}

func (r *Repository) SubmitRun(ctx context.Context, p Principal, key string, payload []byte) (Run, error) {
	payload = bytes.Clone(payload)
	if err := r.principal(p); err != nil {
		return Run{}, err
	}
	fingerprint, err := r.validator.SubmissionFingerprint(payload)
	if err != nil {
		return Run{}, err
	}
	var input runInput
	_ = json.Unmarshal(payload, &input)
	return r.acceptRun(ctx, p, key, payload, input, fingerprint, nil, nil)
}

// Rerun requires the service to confirm terminal status through Temporal.
// checkTerminal is invoked only for a new acceptance, after owner/key lookup;
// matching accepted retries work even if Temporal is now unavailable. A callback
// is not proof of terminal state by itself. No public endpoint invokes this yet.
func (r *Repository) Rerun(ctx context.Context, p Principal, parentID, key string, checkTerminal func(context.Context, Run) error) (Run, error) {
	if err := r.principal(p); err != nil {
		return Run{}, err
	}
	fingerprint, err := r.validator.RerunFingerprint(parentID)
	if err != nil {
		return Run{}, err
	}
	parent, err := r.ownedRun(ctx, p, parentID)
	if err != nil {
		return Run{}, err
	}
	var input runInput
	_ = json.Unmarshal(parent.Input, &input)
	return r.acceptRun(ctx, p, key, parent.Input, input, fingerprint, &parent, checkTerminal)
}

func (r *Repository) acceptRun(ctx context.Context, p Principal, key string, payload []byte, input runInput, fingerprint string, parent *Run, checkTerminal func(context.Context, Run) error) (Run, error) {
	encoded, _ := json.Marshal(key)
	if err := r.validator.Validate("IdempotencyKey", encoded); err != nil {
		return Run{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Run{}, storageError(err)
	}
	defer rollback(tx)
	// Ownership/version association is checked before owner-scoped key lookup.
	var owner string
	var spec []byte
	err = tx.QueryRow(ctx, `SELECT w.owner_id::text,v.spec`+versionJoin+`WHERE `+mutateOwner+`AND w.id=$3 AND v.id=$4`, p.Issuer, p.Subject, input.WorkloadID, input.VersionID).Scan(&owner, &spec)
	if err != nil {
		return Run{}, storageError(err)
	}
	// Hash collisions can only serialize unrelated requests, never merge keys.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, owner+":"+key); err != nil {
		return Run{}, storageError(err)
	}
	var oldFingerprint string
	existing, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+runJoin+`WHERE r.owner_id=$1 AND r.idempotency_key=$2`, owner, key))
	if err == nil {
		err = tx.QueryRow(ctx, `SELECT fingerprint FROM forge.runs WHERE id=$1`, existing.ID).Scan(&oldFingerprint)
		if err != nil {
			return Run{}, storageError(err)
		}
		if oldFingerprint != fingerprint {
			return Run{}, ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Run{}, storageError(err)
		}
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Run{}, err
	}
	var version struct {
		Permissions struct {
			DocumentIDs []string `json:"document_ids"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(spec, &version); err != nil {
		return Run{}, ErrUnavailable
	}
	allowed := make(map[string]bool)
	for _, id := range version.Permissions.DocumentIDs {
		allowed[id] = true
	}
	for _, id := range input.DocumentIDs {
		if !allowed[id] {
			return Run{}, contract.ErrInvalidInput
		}
	}
	var parentID *string
	operation := "createRun"
	if parent != nil {
		if checkTerminal == nil {
			return Run{}, ErrUnavailable
		}
		observed := *parent
		observed.Input = bytes.Clone(parent.Input)
		if err := checkTerminal(ctx, observed); err != nil {
			return Run{}, err
		}
		parentID = &parent.ID
		operation = "rerun"
	}
	id, err := uuid()
	if err != nil {
		return Run{}, err
	}
	commandID, err := uuid()
	if err != nil {
		return Run{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge.runs(id,owner_id,workload_id,version_id,input,idempotency_key,fingerprint,operation,rerun_of,workflow_id)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, owner, input.WorkloadID, input.VersionID, payload, key, fingerprint, operation, parentID, "forge-run/"+id)
	if err != nil {
		return Run{}, storageError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge.commands(id,run_id,kind) VALUES ($1,$2,'start')`, commandID, id)
	if err != nil {
		return Run{}, storageError(err)
	}
	run, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+runJoin+`WHERE r.id=$1`, id))
	if err != nil {
		return Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, storageError(err)
	}
	return run, nil
}

func (r *Repository) GetRun(ctx context.Context, p Principal, id string) (Run, error) {
	if err := r.principal(p); err != nil {
		return Run{}, err
	}
	if err := r.id(id); err != nil {
		return Run{}, err
	}
	return scanRun(r.pool.QueryRow(ctx, `SELECT `+runColumns+runJoin+`WHERE `+readOwner+`AND r.id=$4`, p.Issuer, p.Subject, p.Role, id))
}
func (r *Repository) ownedRun(ctx context.Context, p Principal, id string) (Run, error) {
	if err := r.id(id); err != nil {
		return Run{}, err
	}
	return scanRun(r.pool.QueryRow(ctx, `SELECT `+runColumns+runJoin+`WHERE `+mutateOwner+`AND r.id=$3`, p.Issuer, p.Subject, id))
}
func (r *Repository) ListRuns(ctx context.Context, p Principal, filter RunFilter) ([]Run, error) {
	if err := r.principal(p); err != nil {
		return nil, err
	}
	if err := r.page(filter.Limit, filter.After); err != nil {
		return nil, err
	}
	var workload *string
	if filter.WorkloadID != "" {
		if _, err := r.GetWorkload(ctx, p, filter.WorkloadID); err != nil {
			return nil, err
		}
		workload = &filter.WorkloadID
	}
	var at any
	var id any
	if filter.After != nil {
		at = filter.After.CreatedAt
		id = filter.After.ID
	}
	rows, err := r.pool.Query(ctx, `SELECT `+runColumns+runJoin+`WHERE `+readOwner+`
        AND ($4::uuid IS NULL OR r.workload_id=$4) AND ($5::timestamptz IS NULL OR (r.created_at,r.id)<($5,$6::uuid))
        ORDER BY r.created_at DESC,r.id DESC LIMIT $7`, p.Issuer, p.Subject, p.Role, workload, at, id, filter.Limit)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	out := make([]Run, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, storageError(rows.Err())
}
func scanCommand(row pgx.Row) (Command, error) {
	var c Command
	err := row.Scan(&c.ID, &c.RunID, &c.Kind, &c.State, &c.CreatedAt, &c.DeliveredAt)
	return c, storageError(err)
}

// Cancel durably records one intent even while start is pending. It does not
// claim that Temporal received cancellation or that execution has stopped.
func (r *Repository) Cancel(ctx context.Context, p Principal, runID string) (Command, error) {
	if err := r.principal(p); err != nil {
		return Command{}, err
	}
	if _, err := r.ownedRun(ctx, p, runID); err != nil {
		return Command{}, err
	}
	id, err := uuid()
	if err != nil {
		return Command{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Command{}, storageError(err)
	}
	defer rollback(tx)
	_, err = tx.Exec(ctx, `INSERT INTO forge.commands(id,run_id,kind) VALUES ($1,$2,'cancel') ON CONFLICT (run_id,kind) DO NOTHING`, id, runID)
	if err != nil {
		return Command{}, storageError(err)
	}
	c, err := scanCommand(tx.QueryRow(ctx, `SELECT id::text,run_id::text,kind,state,created_at,delivered_at FROM forge.commands WHERE run_id=$1 AND kind='cancel'`, runID))
	if err != nil {
		return Command{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Command{}, storageError(err)
	}
	return c, nil
}
func (r *Repository) Commands(ctx context.Context, p Principal, runID string) ([]Command, error) {
	if _, err := r.GetRun(ctx, p, runID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT c.id::text,c.run_id::text,c.kind,c.state,c.created_at,c.delivered_at
        FROM forge.commands c JOIN forge.runs r ON r.id=c.run_id JOIN forge.principals p ON p.id=r.owner_id
        WHERE `+readOwner+`AND r.id=$4 ORDER BY CASE c.kind WHEN 'start' THEN 0 ELSE 1 END`, p.Issuer, p.Subject, p.Role, runID)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	out := make([]Command, 0)
	for rows.Next() {
		c, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, storageError(rows.Err())
}
