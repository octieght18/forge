-- Administrator creates forge_environment first. It can verify identities and read open operation columns. It cannot update product rows.
REVOKE ALL ON DATABASE forge FROM forge_environment;
GRANT CONNECT ON DATABASE forge TO forge_environment;
REVOKE ALL ON SCHEMA forge FROM forge_environment;
GRANT USAGE ON SCHEMA forge TO forge_environment;
REVOKE ALL ON ALL TABLES IN SCHEMA forge FROM forge_environment;
GRANT SELECT(id, owner_id) ON forge.workloads TO forge_environment;
GRANT SELECT(id, issuer, subject) ON forge.principals TO forge_environment;
ALTER ROLE forge_environment SET default_transaction_read_only = on;
DO $$
BEGIN
  IF to_regclass('forge.provisioning_operations') IS NOT NULL THEN
    GRANT SELECT (id, workload_id, action, status, desired_generation, observed_generation, observed_phase, deadline, created_at)
      ON forge.provisioning_operations TO forge_environment;
  END IF;
END $$;
