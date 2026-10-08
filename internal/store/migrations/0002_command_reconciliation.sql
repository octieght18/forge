-- Delivery metadata is separate from authoritative Temporal execution state.
ALTER TABLE forge.commands
    ADD COLUMN attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 8),
    ADD COLUMN lease_token uuid,
    ADD COLUMN lease_until timestamptz,
    ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN blocked boolean NOT NULL DEFAULT false,
    ADD COLUMN last_error_code text CHECK (last_error_code IN
        ('retryable','deadline','permanent','invalid_payload','identity_mismatch','attempts_exhausted')),
    ADD CONSTRAINT command_lease_pair CHECK ((lease_token IS NULL) = (lease_until IS NULL)),
    ADD CONSTRAINT command_lease_state CHECK (lease_token IS NULL OR (state = 'pending' AND NOT blocked AND attempts > 0)),
    ADD CONSTRAINT command_delivered_state CHECK (state <> 'delivered' OR (NOT blocked AND lease_token IS NULL AND last_error_code IS NULL));

CREATE INDEX claimable_commands ON forge.commands(next_attempt_at,created_at,id)
    WHERE state = 'pending' AND NOT blocked;

CREATE FUNCTION forge.guard_command_change() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id OR NEW.run_id IS DISTINCT FROM OLD.run_id
       OR NEW.kind IS DISTINCT FROM OLD.kind OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR OLD.state = 'delivered' THEN
        RAISE EXCEPTION 'invalid command identity or delivered command change' USING ERRCODE = '23514';
    END IF;
    IF NEW.state = 'delivered' THEN
        IF OLD.lease_token IS NULL OR OLD.lease_until <= clock_timestamp() THEN
            RAISE EXCEPTION 'command acknowledgement requires active lease' USING ERRCODE = '23514';
        END IF;
        IF NEW.kind = 'cancel' AND NOT EXISTS
            (SELECT 1 FROM forge.commands WHERE run_id = NEW.run_id AND kind = 'start' AND state = 'delivered') THEN
            RAISE EXCEPTION 'cancel requires acknowledged start' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER command_delivery_guard BEFORE UPDATE ON forge.commands
    FOR EACH ROW EXECUTE FUNCTION forge.guard_command_change();
REVOKE ALL ON FUNCTION forge.guard_command_change() FROM PUBLIC;
