package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/contract"
)

var ErrLeaseLost = errors.New("command lease expired or superseded")

// Delivery is privileged background data, never a public response. Token fences
// database acknowledgement, not external effects; the backend must be idempotent.
type Delivery struct {
	CommandID, Kind, Token string
	Attempt                int
	LeaseUntil             time.Time
	Run                    Run
	Spec                   json.RawMessage
	SpecSHA256             string
}

// Dispatcher requires its own pool using the dispatcher role. It is deliberately
// separate from owner-facing Repository methods and carries no caller principal.
type Dispatcher struct {
	pool      *pgxpool.Pool
	validator *contract.Validator
}

func NewDispatcher(pool *pgxpool.Pool) (*Dispatcher, error) {
	if pool == nil {
		return nil, errors.New("dispatcher pool is required")
	}
	v, err := contract.New()
	if err != nil {
		return nil, err
	}
	return &Dispatcher{pool, v}, nil
}

// Claim uses short autocommit statements. No connection/row lock survives into
// backend delivery. Database time controls eligibility and lease expiration.
func (d *Dispatcher) Claim(ctx context.Context, lease time.Duration) (*Delivery, error) {
	if lease < time.Millisecond || lease > time.Minute {
		return nil, contract.ErrInvalidInput
	}
	// A crashed eighth attempt must be parked after its lease expires, too.
	_, err := d.pool.Exec(ctx, `WITH exhausted AS (
 SELECT id FROM forge.commands WHERE state='pending' AND NOT blocked AND attempts=8
 AND (lease_until IS NULL OR lease_until<=clock_timestamp()) ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED)
 UPDATE forge.commands c SET blocked=true,lease_token=NULL,lease_until=NULL,last_error_code='attempts_exhausted'
 FROM exhausted e WHERE c.id=e.id`)
	if err != nil {
		return nil, storageError(err)
	}
	token, err := uuid()
	if err != nil {
		return nil, err
	}
	var x Delivery
	err = d.pool.QueryRow(ctx, `WITH candidate AS (
 SELECT c.id FROM forge.commands c WHERE c.state='pending' AND NOT c.blocked AND c.attempts<8
 AND c.next_attempt_at<=clock_timestamp() AND (c.lease_until IS NULL OR c.lease_until<=clock_timestamp())
 AND (c.kind='start' OR EXISTS (SELECT 1 FROM forge.commands s WHERE s.run_id=c.run_id AND s.kind='start' AND s.state='delivered'))
 ORDER BY c.next_attempt_at,c.created_at,c.id LIMIT 1 FOR UPDATE OF c SKIP LOCKED), claimed AS (
 UPDATE forge.commands c SET attempts=attempts+1,lease_token=$1,lease_until=clock_timestamp()+($2::bigint*interval '1 millisecond')
 FROM candidate q WHERE c.id=q.id
 RETURNING c.id,c.run_id,c.kind,c.lease_token,c.attempts,c.lease_until)
 SELECT x.id::text,x.kind,x.lease_token::text,x.attempts,x.lease_until,
 r.id::text,p.issuer,p.subject,r.workload_id::text,r.version_id::text,r.input,r.rerun_of::text,r.workflow_id,r.created_at,v.spec,v.fingerprint
 FROM claimed x JOIN forge.runs r ON r.id=x.run_id JOIN forge.principals p ON p.id=r.owner_id
 JOIN forge.versions v ON v.id=r.version_id AND v.workload_id=r.workload_id`, token, lease.Milliseconds()).Scan(
		&x.CommandID, &x.Kind, &x.Token, &x.Attempt, &x.LeaseUntil,
		&x.Run.ID, &x.Run.Owner.Issuer, &x.Run.Owner.Subject, &x.Run.WorkloadID, &x.Run.VersionID,
		&x.Run.Input, &x.Run.RerunOf, &x.Run.WorkflowID, &x.Run.CreatedAt, &x.Spec, &x.SpecSHA256)
	if errors.Is(storageError(err), ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, storageError(err)
	}
	return &x, nil
}

func (d *Dispatcher) leaseIdentity(x Delivery) error {
	for _, id := range []string{x.CommandID, x.Token} {
		b, _ := json.Marshal(id)
		if err := d.validator.Validate("ID", b); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dispatcher) Acknowledge(ctx context.Context, x Delivery) error {
	if err := d.leaseIdentity(x); err != nil {
		return err
	}
	result, err := d.pool.Exec(ctx, `UPDATE forge.commands SET state='delivered',delivered_at=clock_timestamp(),
 lease_token=NULL,lease_until=NULL,last_error_code=NULL WHERE id=$1 AND lease_token=$2 AND lease_until>clock_timestamp()
 AND state='pending' AND NOT blocked`, x.CommandID, x.Token)
	if err != nil {
		return storageError(err)
	}
	if result.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}

// Fail records only reviewed codes, never backend error text or payloads. Retry
// backoff is 1,2,4,8,16,32,60 seconds; attempt eight parks the pending intent.
func (d *Dispatcher) Fail(ctx context.Context, x Delivery, code string) error {
	if err := d.leaseIdentity(x); err != nil {
		return err
	}
	switch code {
	case "retryable", "deadline", "permanent", "invalid_payload", "identity_mismatch":
	default:
		return contract.ErrInvalidInput
	}
	result, err := d.pool.Exec(ctx, `UPDATE forge.commands SET
 blocked=(attempts>=8 OR $3 NOT IN ('retryable','deadline')),
 last_error_code=CASE WHEN attempts>=8 THEN 'attempts_exhausted' ELSE $3 END,
 next_attempt_at=clock_timestamp()+(LEAST(60,power(2,attempts-1))*interval '1 second'),
 lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2 AND lease_until>clock_timestamp()
 AND state='pending' AND NOT blocked`, x.CommandID, x.Token, code)
	if err != nil {
		return storageError(err)
	}
	if result.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}
