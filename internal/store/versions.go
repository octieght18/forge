package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/octieght18/forge/internal/contract"
)

const versionColumns = `v.id::text,v.workload_id::text,v.number,v.spec,v.fingerprint,v.created_at`
const versionJoin = ` FROM forge.versions v JOIN forge.workloads w ON w.id=v.workload_id JOIN forge.principals p ON p.id=w.owner_id `

func scanVersion(row pgx.Row) (Version, error) {
	var v Version
	err := row.Scan(&v.ID, &v.WorkloadID, &v.Number, &v.Spec, &v.Fingerprint, &v.CreatedAt)
	return v, storageError(err)
}

// CreateVersion validates the declaration. Corpus existence and trusted source
// approval remain obligations of the service before calling this repository.
func (r *Repository) CreateVersion(ctx context.Context, p Principal, workloadID string, payload []byte) (Version, error) {
	if err := r.principal(p); err != nil {
		return Version{}, err
	}
	if err := r.id(workloadID); err != nil {
		return Version{}, err
	}
	fingerprint, err := r.validator.VersionFingerprint(payload)
	if err != nil {
		return Version{}, err
	}
	var input struct {
		Spec json.RawMessage `json:"spec"`
	}
	_ = json.Unmarshal(payload, &input)
	id, err := uuid()
	if err != nil {
		return Version{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Version{}, storageError(err)
	}
	defer rollback(tx)
	// Lock one owned workload so MAX+1 is safe across concurrent registrations.
	var locked string
	err = tx.QueryRow(ctx, `SELECT w.id::text`+workloadJoin+`WHERE `+mutateOwner+`AND w.id=$3 FOR UPDATE OF w`, p.Issuer, p.Subject, workloadID).Scan(&locked)
	if err != nil {
		return Version{}, storageError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge.versions(id,workload_id,number,spec,fingerprint)
        SELECT $1,$2,coalesce(max(number),0)+1,$3,$4 FROM forge.versions WHERE workload_id=$2`, id, workloadID, input.Spec, fingerprint)
	if err != nil {
		return Version{}, storageError(err)
	}
	v, err := scanVersion(tx.QueryRow(ctx, `SELECT `+versionColumns+` FROM forge.versions v WHERE v.id=$1`, id))
	if err != nil {
		return Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, storageError(err)
	}
	return v, nil
}
func (r *Repository) GetVersion(ctx context.Context, p Principal, workloadID, versionID string) (Version, error) {
	if err := r.principal(p); err != nil {
		return Version{}, err
	}
	if err := r.id(workloadID); err != nil {
		return Version{}, err
	}
	if err := r.id(versionID); err != nil {
		return Version{}, err
	}
	return scanVersion(r.pool.QueryRow(ctx, `SELECT `+versionColumns+versionJoin+`WHERE `+readOwner+`AND w.id=$4 AND v.id=$5`, p.Issuer, p.Subject, p.Role, workloadID, versionID))
}

// beforeNumber is an internal cursor position; zero means the first page.
func (r *Repository) ListVersions(ctx context.Context, p Principal, workloadID string, limit int, beforeNumber int64) ([]Version, error) {
	if err := r.principal(p); err != nil {
		return nil, err
	}
	if err := r.id(workloadID); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || beforeNumber < 0 {
		return nil, contract.ErrInvalidInput
	}
	// Missing and inaccessible nested parents both return not found, not an empty page.
	if _, err := r.GetWorkload(ctx, p, workloadID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+versionColumns+versionJoin+`WHERE `+readOwner+`AND w.id=$4
        AND ($5::bigint=0 OR v.number<$5) ORDER BY v.number DESC LIMIT $6`, p.Issuer, p.Subject, p.Role, workloadID, beforeNumber, limit)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	out := make([]Version, 0)
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, storageError(rows.Err())
}
