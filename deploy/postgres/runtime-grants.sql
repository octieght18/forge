-- Run as forge_migrator against the Forge database after forge-migrate succeeds.
GRANT USAGE ON SCHEMA forge TO forge_runtime;
GRANT SELECT, INSERT ON forge.principals, forge.workloads, forge.versions, forge.runs, forge.commands TO forge_runtime;
GRANT UPDATE(name, description, revision, updated_at) ON forge.workloads TO forge_runtime;
-- No DELETE, schema creation, migration ledger access or immutable-record UPDATE.
-- Future tables/command-delivery methods require explicitly reviewed new grants.
