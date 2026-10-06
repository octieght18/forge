-- Run once as a local PostgreSQL administrator in an empty installation.
-- These roles deliberately have no passwords here; set them with psql \password.
CREATE ROLE forge_migrator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE forge_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE DATABASE forge OWNER forge_migrator ENCODING 'UTF8';
REVOKE ALL ON DATABASE forge FROM PUBLIC;
GRANT CONNECT ON DATABASE forge TO forge_migrator, forge_runtime;
