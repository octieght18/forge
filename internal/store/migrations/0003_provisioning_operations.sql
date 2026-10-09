-- Asynchronous provisioning intent. The controller still owns namespace
-- reconciliation; this table records the accepted operation and its observation.
CREATE TABLE forge.provisioning_operations (
    id uuid PRIMARY KEY CHECK (substr(id::text, 15, 1) = '4'),
    workload_id uuid NOT NULL REFERENCES forge.workloads(id),
    version_id uuid NOT NULL REFERENCES forge.versions(id),
    action text NOT NULL CHECK (action IN ('provision', 'delete')),
    desired_generation bigint NOT NULL CHECK (desired_generation > 0),
    observed_generation bigint NOT NULL DEFAULT 0 CHECK (observed_generation >= 0 AND observed_generation <= desired_generation),
    status text NOT NULL CHECK (status IN ('accepted', 'provisioning', 'ready', 'failed', 'cancel_requested', 'canceled', 'timed_out')),
    observed_phase text NOT NULL DEFAULT '' CHECK (char_length(observed_phase) <= 64),
    error_code text CHECK (error_code IS NULL OR error_code IN ('ownership_conflict', 'identity_mismatch', 'controller_failed', 'canceled', 'operation_timeout')),
    error_message text CHECK (error_message IS NULL OR char_length(error_message) BETWEEN 1 AND 512),
    deadline timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT provisioning_terminal_shape CHECK (
        (status = 'ready' AND action = 'provision' AND observed_generation = desired_generation AND observed_phase = 'Ready' AND error_code IS NULL AND error_message IS NULL)
        OR (status = 'failed' AND observed_generation = desired_generation AND observed_phase = 'Failed' AND error_code IN ('ownership_conflict', 'identity_mismatch', 'controller_failed') AND error_message IS NOT NULL)
        OR (status = 'canceled' AND action = 'delete' AND observed_generation = desired_generation AND observed_phase = 'Absent' AND error_code = 'canceled' AND error_message IS NOT NULL)
        OR (status = 'timed_out' AND error_code = 'operation_timeout' AND error_message IS NOT NULL)
        OR (status IN ('accepted', 'provisioning', 'cancel_requested') AND error_code IS NULL AND error_message IS NULL)
    ),
    CONSTRAINT provisioning_cancel_shape CHECK (status <> 'cancel_requested' OR action = 'delete')
);

CREATE UNIQUE INDEX provisioning_operations_one_open ON forge.provisioning_operations(workload_id)
    WHERE status IN ('accepted', 'provisioning', 'cancel_requested');
CREATE INDEX provisioning_operations_workload ON forge.provisioning_operations(workload_id, desired_generation DESC);
