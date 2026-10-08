-- Existing installations only: administrator creates the new role once.
-- Set its password separately with psql \password. Do not use migration/API credentials.
CREATE ROLE forge_dispatcher LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
GRANT CONNECT ON DATABASE forge TO forge_dispatcher;
