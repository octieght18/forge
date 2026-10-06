CREATE TABLE forge.principals (
    id uuid PRIMARY KEY CHECK (substr(id::text, 15, 1) = '4'),
    issuer text COLLATE "C" NOT NULL CHECK (char_length(issuer) BETWEEN 1 AND 2048),
    subject text COLLATE "C" NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 255),
    issuer_hash bytea NOT NULL CHECK (issuer_hash = sha256(convert_to(issuer, 'UTF8'))),
    subject_hash bytea NOT NULL CHECK (subject_hash = sha256(convert_to(subject, 'UTF8'))),
    UNIQUE (issuer_hash, subject_hash)
);
-- Hashes keep maximum-length OIDC identifiers within PostgreSQL index limits.
-- Repository lookup also compares the original strings; a collision fails closed.

CREATE TABLE forge.workloads (
    id uuid PRIMARY KEY CHECK (substr(id::text, 15, 1) = '4'),
    owner_id uuid NOT NULL REFERENCES forge.principals(id),
    name text COLLATE "C" NOT NULL CHECK (name ~ '^[a-z][a-z0-9-]{0,62}$'),
    description text NOT NULL CHECK (char_length(description) <= 1024),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (owner_id, name),
    UNIQUE (id, owner_id)
);
CREATE INDEX workloads_owner_page ON forge.workloads(owner_id, created_at DESC, id DESC);
CREATE INDEX workloads_all_page ON forge.workloads(created_at DESC, id DESC);

CREATE TABLE forge.versions (
    id uuid PRIMARY KEY CHECK (substr(id::text, 15, 1) = '4'),
    workload_id uuid NOT NULL REFERENCES forge.workloads(id),
    number bigint NOT NULL CHECK (number > 0),
    spec jsonb NOT NULL CHECK (jsonb_typeof(spec) = 'object'),
    fingerprint text NOT NULL CHECK (fingerprint ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (workload_id, number),
    UNIQUE (id, workload_id)
);

CREATE TABLE forge.runs (
    id uuid PRIMARY KEY CHECK (substr(id::text, 15, 1) = '4'),
    owner_id uuid NOT NULL,
    workload_id uuid NOT NULL,
    version_id uuid NOT NULL,
    input jsonb NOT NULL CHECK (jsonb_typeof(input) = 'object'),
    idempotency_key text COLLATE "C" NOT NULL CHECK (idempotency_key ~ '^[A-Za-z0-9._:-]{1,128}$'),
    fingerprint text NOT NULL CHECK (fingerprint ~ '^[0-9a-f]{64}$'),
    operation text NOT NULL CHECK (operation IN ('createRun', 'rerun')),
    rerun_of uuid,
    workflow_id text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (owner_id, idempotency_key),
    UNIQUE (id, owner_id, workload_id),
    FOREIGN KEY (workload_id, owner_id) REFERENCES forge.workloads(id, owner_id),
    FOREIGN KEY (version_id, workload_id) REFERENCES forge.versions(id, workload_id),
    FOREIGN KEY (rerun_of, owner_id, workload_id) REFERENCES forge.runs(id, owner_id, workload_id),
    CHECK ((operation = 'createRun' AND rerun_of IS NULL) OR (operation = 'rerun' AND rerun_of IS NOT NULL)),
    CHECK (workflow_id = 'forge-run/' || id::text),
    CHECK (coalesce(input->>'workload_id' = workload_id::text AND input->>'version_id' = version_id::text, false))
);
CREATE INDEX runs_owner_page ON forge.runs(owner_id, created_at DESC, id DESC);
CREATE INDEX runs_workload_page ON forge.runs(workload_id, created_at DESC, id DESC);
CREATE INDEX runs_all_page ON forge.runs(created_at DESC, id DESC);

CREATE TABLE forge.commands (
    id uuid PRIMARY KEY CHECK (substr(id::text, 15, 1) = '4'),
    run_id uuid NOT NULL REFERENCES forge.runs(id),
    kind text NOT NULL CHECK (kind IN ('start', 'cancel')),
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'delivered')),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    delivered_at timestamptz,
    UNIQUE (run_id, kind),
    CHECK ((state = 'pending' AND delivered_at IS NULL) OR (state = 'delivered' AND delivered_at IS NOT NULL))
);
CREATE INDEX pending_commands ON forge.commands(created_at, id) WHERE state = 'pending';

CREATE FUNCTION forge.reject_immutable_change() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog AS $$
BEGIN
    RAISE EXCEPTION 'immutable product record' USING ERRCODE = '23514';
END;
$$;
CREATE TRIGGER immutable_principal BEFORE UPDATE OR DELETE ON forge.principals
    FOR EACH ROW EXECUTE FUNCTION forge.reject_immutable_change();
CREATE TRIGGER immutable_version BEFORE UPDATE OR DELETE ON forge.versions
    FOR EACH ROW EXECUTE FUNCTION forge.reject_immutable_change();
CREATE TRIGGER immutable_run BEFORE UPDATE OR DELETE ON forge.runs
    FOR EACH ROW EXECUTE FUNCTION forge.reject_immutable_change();

CREATE FUNCTION forge.guard_workload_change() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id OR NEW.owner_id IS DISTINCT FROM OLD.owner_id
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'invalid workload revision or identity change' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER workload_revision BEFORE UPDATE ON forge.workloads
    FOR EACH ROW EXECUTE FUNCTION forge.guard_workload_change();

REVOKE ALL ON ALL TABLES IN SCHEMA forge FROM PUBLIC;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA forge FROM PUBLIC;
