-- Run as forge_migrator against the Forge database after forge-migrate succeeds.
GRANT USAGE ON SCHEMA forge TO forge_runtime;
GRANT SELECT, INSERT ON forge.principals, forge.workloads, forge.versions, forge.runs TO forge_runtime;
GRANT SELECT ON forge.commands TO forge_runtime;
-- Revoke the earlier table-wide producer grant on retained installations.
REVOKE INSERT ON forge.commands FROM forge_runtime;
GRANT INSERT(id,run_id,kind) ON forge.commands TO forge_runtime;
GRANT UPDATE(name, description, revision, updated_at) ON forge.workloads TO forge_runtime;
GRANT SELECT, INSERT ON forge.provisioning_operations TO forge_runtime;
GRANT UPDATE(action, desired_generation, observed_generation, status, observed_phase, error_code, error_message, deadline, updated_at) ON forge.provisioning_operations TO forge_runtime;
-- No DELETE, schema creation, migration ledger access or immutable-record UPDATE.
-- Command delivery uses the separate dispatcher role, never this API role.
