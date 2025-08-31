-- 0004_orgs_and_fk.sql

-- ORGANIZATIONS ---------------------------------------------------
CREATE TABLE IF NOT EXISTS organizations (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- Add org_id column to existing tables (idempotent, with default tenant = 1)
ALTER TABLE sites     ADD COLUMN IF NOT EXISTS org_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE vendors   ADD COLUMN IF NOT EXISTS org_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE projects  ADD COLUMN IF NOT EXISTS org_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE inventory ADD COLUMN IF NOT EXISTS org_id BIGINT NOT NULL DEFAULT 1;

-- Indexes for org_id lookups
CREATE INDEX IF NOT EXISTS idx_sites_org_id    ON sites(org_id, id);
CREATE INDEX IF NOT EXISTS idx_vendors_org_id  ON vendors(org_id, id);
CREATE INDEX IF NOT EXISTS idx_projects_org_id ON projects(org_id, id);
CREATE INDEX IF NOT EXISTS idx_items_org_id    ON inventory(org_id, id);

-- Per-organization uniqueness for project code
CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_org_code ON projects(org_id, code);

