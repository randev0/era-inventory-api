-- 0009_assets_core.sql - Core assets table for hybrid asset management

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS assets (
  id          BIGSERIAL PRIMARY KEY,
  org_id      BIGINT NOT NULL REFERENCES organizations(id),
  site_id     BIGINT NOT NULL REFERENCES sites(id),
  asset_type  TEXT NOT NULL,              -- e.g. "switch","firewall","ap","vlan","peplink","software"
  name        TEXT,
  vendor      TEXT,
  model       TEXT,
  serial      TEXT,
  mgmt_ip     INET,
  status      TEXT,
  notes       TEXT,
  extras      JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_assets_serial_per_site
  ON assets(org_id, site_id, asset_type, serial)
  WHERE serial IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_assets_org_site   ON assets(org_id, site_id, id);
CREATE INDEX IF NOT EXISTS idx_assets_type       ON assets(org_id, site_id, asset_type, id);
CREATE INDEX IF NOT EXISTS idx_assets_name_trgm  ON assets USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_assets_extras_gin ON assets USING gin (extras jsonb_path_ops);

-- Trigger for updated_at
DROP TRIGGER IF EXISTS trg_assets_updated_at ON assets;
CREATE TRIGGER trg_assets_updated_at
    BEFORE UPDATE ON assets
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
