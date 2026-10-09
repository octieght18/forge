package store

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CheckSchema checks required product tables and runtime read privileges without
// reading data or accessing the migration ledger. It never applies migrations.
func CheckSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `SELECT id FROM forge.principals LIMIT 0;
 SELECT id,name,description,revision,created_at,updated_at FROM forge.workloads LIMIT 0;
 SELECT id,workload_id,number,spec,fingerprint,created_at FROM forge.versions LIMIT 0;
 SELECT id FROM forge.runs LIMIT 0; SELECT id FROM forge.commands LIMIT 0;
 SELECT id,workload_id,version_id,action,desired_generation,observed_generation,status,observed_phase,error_code,error_message,deadline,updated_at FROM forge.provisioning_operations LIMIT 0;`)
	return storageError(err)
}
