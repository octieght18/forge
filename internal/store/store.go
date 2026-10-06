// Package store persists Forge product state. Principals must come from a trusted
// authentication/policy layer. It does not validate OIDC tokens or execute workflows.
package store

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/contract"
)

var (
	ErrNotFound     = errors.New("product record not found")
	ErrConflict     = errors.New("product record conflict")
	ErrPrecondition = errors.New("workload revision does not match")
	ErrForbidden    = errors.New("principal role is not permitted")
	ErrUnavailable  = errors.New("product storage unavailable")
)

type Principal struct{ Issuer, Subject, Role string }
type Owner struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}
type Workload struct {
	ID          string    `json:"workload_id"`
	Owner       Owner     `json:"owner"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Revision    int64     `json:"revision"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type Version struct {
	ID          string          `json:"version_id"`
	WorkloadID  string          `json:"workload_id"`
	Number      int64           `json:"version_number"`
	Spec        json.RawMessage `json:"spec"`
	Fingerprint string          `json:"spec_sha256"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Run contains stored submission data, never an authoritative execution status.
// The HTTP layer must obtain Temporal observations and command delivery state.
type Run struct {
	ID                    string
	Owner                 Owner
	WorkloadID, VersionID string
	Input                 json.RawMessage
	RerunOf               *string
	WorkflowID            string
	CreatedAt             time.Time
}
type Command struct {
	ID, RunID, Kind, State string
	CreatedAt              time.Time
	DeliveredAt            *time.Time
}

// Position is an internal keyset position, not an authenticated API cursor.
type Position struct {
	CreatedAt time.Time
	ID        string
}
type RunFilter struct {
	WorkloadID string
	After      *Position
	Limit      int
}
type Repository struct {
	pool      *pgxpool.Pool
	validator *contract.Validator
}

func New(pool *pgxpool.Pool) (*Repository, error) {
	if pool == nil {
		return nil, errors.New("database pool is required")
	}
	validator, err := contract.New()
	if err != nil {
		return nil, err
	}
	return &Repository{pool: pool, validator: validator}, nil
}

func (r *Repository) principal(p Principal) error {
	if p.Role != "developer" && p.Role != "operator" {
		return ErrForbidden
	}
	b, _ := json.Marshal(Owner{p.Issuer, p.Subject})
	return r.validator.Validate("Owner", b)
}
func (r *Repository) id(id string) error {
	b, _ := json.Marshal(id)
	return r.validator.Validate("ID", b)
}
func (r *Repository) page(limit int, after *Position) error {
	if limit < 1 || limit > 100 {
		return contract.ErrInvalidInput
	}
	if after != nil {
		if after.CreatedAt.IsZero() {
			return contract.ErrInvalidInput
		}
		return r.id(after.ID)
	}
	return nil
}
func uuid() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", ErrUnavailable
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
func storageError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return ErrConflict
		case "23503", "23514", "22P02", "22P05", "22001", "22021":
			return contract.ErrInvalidInput
		}
	}
	return ErrUnavailable
}

// ownerID deduplicates immutable issuer/subject identities. Check exact strings
// after the hash-index conflict; a hypothetical digest collision never grants access.
func ownerID(ctx context.Context, tx pgx.Tx, p Principal) (string, error) {
	id, err := uuid()
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge.principals(id,issuer,subject,issuer_hash,subject_hash)
        VALUES ($1,$2,$3,sha256(convert_to($2::text,'UTF8')),sha256(convert_to($3::text,'UTF8'))) ON CONFLICT DO NOTHING`, id, p.Issuer, p.Subject)
	if err != nil {
		return "", storageError(err)
	}
	err = tx.QueryRow(ctx, `SELECT id::text FROM forge.principals WHERE
		issuer_hash=sha256(convert_to($1::text,'UTF8')) AND
		subject_hash=sha256(convert_to($2::text,'UTF8')) AND issuer=$1 AND subject=$2`, p.Issuer, p.Subject).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrConflict
	}
	return id, storageError(err)
}

const workloadColumns = `w.id::text,p.issuer,p.subject,w.name,w.description,w.revision,w.created_at,w.updated_at`
const workloadJoin = ` FROM forge.workloads w JOIN forge.principals p ON p.id=w.owner_id `
const readOwner = ` (p.issuer=$1 AND p.subject=$2 OR $3::text='operator') `
const mutateOwner = ` p.issuer=$1 AND p.subject=$2 `

func scanWorkload(row pgx.Row) (Workload, error) {
	var w Workload
	err := row.Scan(&w.ID, &w.Owner.Issuer, &w.Owner.Subject, &w.Name, &w.Description, &w.Revision, &w.CreatedAt, &w.UpdatedAt)
	return w, storageError(err)
}

func (r *Repository) CreateWorkload(ctx context.Context, p Principal, payload []byte) (Workload, error) {
	if err := r.principal(p); err != nil {
		return Workload{}, err
	}
	if err := r.validator.ValidateRequest("CreateWorkloadRequest", payload); err != nil {
		return Workload{}, err
	}
	var input struct{ Name, Description string }
	_ = json.Unmarshal(payload, &input)
	id, err := uuid()
	if err != nil {
		return Workload{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Workload{}, storageError(err)
	}
	defer rollback(tx)
	owner, err := ownerID(ctx, tx, p)
	if err != nil {
		return Workload{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO forge.workloads(id,owner_id,name,description) VALUES ($1,$2,$3,$4)`, id, owner, input.Name, input.Description)
	if err != nil {
		return Workload{}, storageError(err)
	}
	w, err := scanWorkload(tx.QueryRow(ctx, `SELECT `+workloadColumns+workloadJoin+`WHERE w.id=$1`, id))
	if err != nil {
		return Workload{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Workload{}, storageError(err)
	}
	return w, nil
}
func (r *Repository) GetWorkload(ctx context.Context, p Principal, id string) (Workload, error) {
	if err := r.principal(p); err != nil {
		return Workload{}, err
	}
	if err := r.id(id); err != nil {
		return Workload{}, err
	}
	return scanWorkload(r.pool.QueryRow(ctx, `SELECT `+workloadColumns+workloadJoin+`WHERE `+readOwner+`AND w.id=$4`, p.Issuer, p.Subject, p.Role, id))
}
func (r *Repository) ListWorkloads(ctx context.Context, p Principal, limit int, after *Position) ([]Workload, error) {
	if err := r.principal(p); err != nil {
		return nil, err
	}
	if err := r.page(limit, after); err != nil {
		return nil, err
	}
	var at *time.Time
	var id *string
	if after != nil {
		at = &after.CreatedAt
		id = &after.ID
	}
	rows, err := r.pool.Query(ctx, `SELECT `+workloadColumns+workloadJoin+`WHERE `+readOwner+`
        AND ($4::timestamptz IS NULL OR (w.created_at,w.id)<($4,$5::uuid)) ORDER BY w.created_at DESC,w.id DESC LIMIT $6`, p.Issuer, p.Subject, p.Role, at, id, limit)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	out := make([]Workload, 0)
	for rows.Next() {
		w, err := scanWorkload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, storageError(rows.Err())
}
func (r *Repository) UpdateWorkload(ctx context.Context, p Principal, id string, revision int64, payload []byte) (Workload, error) {
	if err := r.principal(p); err != nil {
		return Workload{}, err
	}
	if err := r.id(id); err != nil {
		return Workload{}, err
	}
	if revision < 1 {
		return Workload{}, contract.ErrInvalidInput
	}
	if err := r.validator.ValidateRequest("UpdateWorkloadRequest", payload); err != nil {
		return Workload{}, err
	}
	var patch struct{ Name, Description *string }
	_ = json.Unmarshal(payload, &patch)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Workload{}, storageError(err)
	}
	defer rollback(tx)
	w, err := scanWorkload(tx.QueryRow(ctx, `SELECT `+workloadColumns+workloadJoin+`WHERE `+mutateOwner+`AND w.id=$3 FOR UPDATE OF w`, p.Issuer, p.Subject, id))
	if err != nil {
		return Workload{}, err
	}
	if w.Revision != revision {
		return Workload{}, ErrPrecondition
	}
	_, err = tx.Exec(ctx, `UPDATE forge.workloads SET name=coalesce($1,name),description=coalesce($2,description),revision=revision+1,updated_at=clock_timestamp() WHERE id=$3 AND revision=$4`, patch.Name, patch.Description, id, revision)
	if err != nil {
		return Workload{}, storageError(err)
	}
	w, err = scanWorkload(tx.QueryRow(ctx, `SELECT `+workloadColumns+workloadJoin+`WHERE w.id=$1`, id))
	if err != nil {
		return Workload{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Workload{}, storageError(err)
	}
	return w, nil
}
