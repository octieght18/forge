package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/octieght18/forge/internal/provision"
)

const operationColumns = `o.id::text,o.workload_id::text,o.version_id::text,o.action,o.desired_generation,o.observed_generation,o.status,o.observed_phase,COALESCE(o.error_code,''),COALESCE(o.error_message,''),o.deadline`

func scanOperation(row pgx.Row) (provision.Operation, error) {
	var op provision.Operation
	var status string
	err := row.Scan(&op.ID, &op.WorkloadID, &op.VersionID, &op.Action, &op.DesiredGeneration, &op.ObservedGeneration, &status, &op.ObservedPhase, &op.ErrorCode, &op.ErrorMessage, &op.Deadline)
	op.Status = provision.Status(status)
	return op, storageError(err)
}

func (r *Repository) AcceptOperation(ctx context.Context, p Principal, workloadID, versionID, action string, timeout time.Duration, now time.Time) (provision.Operation, error) {
	if p.Role != "developer" {
		return provision.Operation{}, ErrForbidden
	}
	if err := r.principal(p); err != nil {
		return provision.Operation{}, err
	}
	if err := r.id(workloadID); err != nil {
		return provision.Operation{}, err
	}
	if err := r.id(versionID); err != nil {
		return provision.Operation{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return provision.Operation{}, storageError(err)
	}
	defer rollback(tx)
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM forge.workloads w JOIN forge.principals p ON p.id=w.owner_id WHERE `+mutateOwner+` AND w.id=$3) `, p.Issuer, p.Subject, workloadID).Scan(&exists)
	if err != nil {
		return provision.Operation{}, storageError(err)
	}
	if !exists {
		return provision.Operation{}, ErrNotFound
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM forge.workloads WHERE id=$1 FOR UPDATE`, workloadID); err != nil {
		return provision.Operation{}, storageError(err)
	}
	var latest string
	err = tx.QueryRow(ctx, `SELECT id::text FROM forge.versions WHERE workload_id=$1 ORDER BY number DESC LIMIT 1`, workloadID).Scan(&latest)
	if err != nil {
		latest = ""
		if storageError(err) != ErrNotFound {
			return provision.Operation{}, storageError(err)
		}
	}
	var open bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM forge.provisioning_operations WHERE workload_id=$1 AND status IN ('accepted','provisioning','cancel_requested'))`, workloadID).Scan(&open); err != nil {
		return provision.Operation{}, storageError(err)
	}
	var generation int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(desired_generation),0)+1 FROM forge.provisioning_operations WHERE workload_id=$1`, workloadID).Scan(&generation); err != nil {
		return provision.Operation{}, storageError(err)
	}
	op, err := provision.Accept(latest, versionID, action, generation, open, now, timeout)
	if err != nil {
		return provision.Operation{}, err
	}
	op.ID, err = uuid()
	if err != nil {
		return provision.Operation{}, err
	}
	op.WorkloadID = workloadID
	_, err = tx.Exec(ctx, `INSERT INTO forge.provisioning_operations(id,workload_id,version_id,action,desired_generation,status,deadline)
		VALUES ($1,$2,$3,$4,$5,'accepted',$6)`, op.ID, workloadID, versionID, action, generation, op.Deadline)
	if err != nil {
		return provision.Operation{}, storageError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return provision.Operation{}, storageError(err)
	}
	return op, nil
}

func (r *Repository) GetOperation(ctx context.Context, p Principal, id string, now time.Time) (provision.Operation, error) {
	if err := r.principal(p); err != nil {
		return provision.Operation{}, err
	}
	if err := r.id(id); err != nil {
		return provision.Operation{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return provision.Operation{}, storageError(err)
	}
	defer rollback(tx)
	op, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+` FROM forge.provisioning_operations o
		JOIN forge.workloads w ON w.id=o.workload_id JOIN forge.principals p ON p.id=w.owner_id
		WHERE `+readOwner+` AND o.id=$4 FOR UPDATE OF o`, p.Issuer, p.Subject, p.Role, id))
	if err != nil {
		return provision.Operation{}, err
	}
	next := provision.Advance(op, nil, now)
	if err = saveOperation(ctx, tx, op, next); err != nil {
		return provision.Operation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return provision.Operation{}, storageError(err)
	}
	return next, nil
}

func (r *Repository) CancelOperation(ctx context.Context, p Principal, id string, now time.Time) (provision.Operation, error) {
	if p.Role != "developer" {
		return provision.Operation{}, ErrForbidden
	}
	if err := r.principal(p); err != nil {
		return provision.Operation{}, err
	}
	if err := r.id(id); err != nil {
		return provision.Operation{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return provision.Operation{}, storageError(err)
	}
	defer rollback(tx)
	op, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+` FROM forge.provisioning_operations o
		JOIN forge.workloads w ON w.id=o.workload_id JOIN forge.principals p ON p.id=w.owner_id
		WHERE `+mutateOwner+` AND o.id=$3 FOR UPDATE OF o`, p.Issuer, p.Subject, id))
	if err != nil {
		return provision.Operation{}, err
	}
	next, err := provision.Cancel(op, now, provision.DefaultTimeout)
	if err != nil {
		return provision.Operation{}, err
	}
	if err = saveOperation(ctx, tx, op, next); err != nil {
		return provision.Operation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return provision.Operation{}, storageError(err)
	}
	return next, nil
}

// ObserveOperation applies one controller phase to the workload's open operation.
// No open operation is a no-op so a controller pass can report before the API accepts work.
func (r *Repository) ObserveOperation(ctx context.Context, workloadID string, obs provision.Observation, now time.Time) error {
	if err := r.id(workloadID); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return storageError(err)
	}
	defer rollback(tx)
	op, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+` FROM forge.provisioning_operations o
		WHERE o.workload_id=$1 AND o.status IN ('accepted','provisioning','cancel_requested')
		ORDER BY o.desired_generation DESC LIMIT 1 FOR UPDATE`, workloadID))
	if err != nil {
		if err == ErrNotFound {
			return tx.Commit(ctx)
		}
		return err
	}
	next := provision.Advance(op, &obs, now)
	if err = saveOperation(ctx, tx, op, next); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func saveOperation(ctx context.Context, tx pgx.Tx, before, next provision.Operation) error {
	if before == next {
		return nil
	}
	var code, message any
	if next.ErrorCode != "" {
		code = next.ErrorCode
	}
	if next.ErrorMessage != "" {
		message = next.ErrorMessage
	}
	_, err := tx.Exec(ctx, `UPDATE forge.provisioning_operations SET action=$2, desired_generation=$3, observed_generation=$4,
		status=$5, observed_phase=$6, error_code=$7, error_message=$8, deadline=$9, updated_at=clock_timestamp() WHERE id=$1`,
		next.ID, next.Action, next.DesiredGeneration, next.ObservedGeneration, string(next.Status), next.ObservedPhase, code, message, next.Deadline)
	return storageError(err)
}
