-- Existing demo rows are owned by issuer and subject. This migration does not
-- delete them. The demo tenant and project are the forward-filled boundary.
CREATE TABLE forge.tenants (
    id uuid PRIMARY KEY CHECK (substr(id::text, 15, 1) = '4'),
    name text NOT NULL UNIQUE CHECK (name ~ '^[a-z][a-z0-9-]{0,62}$')
);
CREATE TABLE forge.projects (
    id uuid PRIMARY KEY CHECK (substr(id::text, 15, 1) = '4'),
    tenant_id uuid NOT NULL REFERENCES forge.tenants(id),
    name text NOT NULL CHECK (name ~ '^[a-z][a-z0-9-]{0,62}$'),
    UNIQUE (tenant_id, name),
    UNIQUE (id, tenant_id)
);
INSERT INTO forge.tenants(id, name) VALUES ('00000000-0000-4000-8000-000000000001', 'demo');
INSERT INTO forge.projects(id, tenant_id, name) VALUES (
    '00000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000001',
    'demo'
);

ALTER TABLE forge.workloads
    ADD COLUMN tenant_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000001' REFERENCES forge.tenants(id),
    ADD COLUMN project_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000002';
ALTER TABLE forge.workloads
    ADD FOREIGN KEY (project_id, tenant_id) REFERENCES forge.projects(id, tenant_id);

ALTER TABLE forge.versions
    ADD COLUMN tenant_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000001' REFERENCES forge.tenants(id),
    ADD COLUMN project_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000002';
ALTER TABLE forge.versions
    ADD FOREIGN KEY (project_id, tenant_id) REFERENCES forge.projects(id, tenant_id);

ALTER TABLE forge.runs
    ADD COLUMN tenant_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000001' REFERENCES forge.tenants(id),
    ADD COLUMN project_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000002';
ALTER TABLE forge.runs
    ADD FOREIGN KEY (project_id, tenant_id) REFERENCES forge.projects(id, tenant_id);
