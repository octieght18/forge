-- Run as forge_migrator after migrations. This is a privileged background role,
-- never the API's role or a developer credential. No product INSERT/DELETE/UPDATE.
GRANT USAGE ON SCHEMA forge TO forge_dispatcher;
GRANT SELECT ON forge.principals,forge.workloads,forge.versions,forge.runs,forge.commands TO forge_dispatcher;
GRANT UPDATE(state,delivered_at,attempts,lease_token,lease_until,next_attempt_at,blocked,last_error_code)
    ON forge.commands TO forge_dispatcher;
