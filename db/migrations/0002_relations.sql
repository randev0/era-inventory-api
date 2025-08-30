-- 0002_relations.sql
-- Reference tables for relational model (sites, vendors, projects)
-- Reuses the set_updated_at() trigger function from 0001.

-- SITES ----------------------------------------------------------
CREATE TABLE IF NOT EXISTS sites (
  id          SERIAL PRIMARY KEY,
  name        TEXT NOT NULL,
  code        TEXT UNIQUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS trg_sites_updated_at ON sites;
CREATE TRIGGER trg_sites_updated_at
BEFORE UPDATE ON sites
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_sites_code ON sites(code);

-- VENDORS --------------------------------------------------------
CREATE TABLE IF NOT EXISTS vendors (
  id          SERIAL PRIMARY KEY,
  name        TEXT NOT NULL,
  contact     TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS trg_vendors_updated_at ON vendors;
CREATE TRIGGER trg_vendors_updated_at
BEFORE UPDATE ON vendors
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_vendors_name ON vendors(name);

-- PROJECTS -------------------------------------------------------
CREATE TABLE IF NOT EXISTS projects (
  id          SERIAL PRIMARY KEY,
  name        TEXT NOT NULL,
  code        TEXT UNIQUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS trg_projects_updated_at ON projects;
CREATE TRIGGER trg_projects_updated_at
BEFORE UPDATE ON projects
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_projects_code ON projects(code);
