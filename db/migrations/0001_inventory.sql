-- 0001_inventory.sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS inventory (
    id SERIAL PRIMARY KEY,
    asset_tag TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    manufacturer TEXT,
    model TEXT,
    device_type TEXT,
    site TEXT,
    installed_at DATE,
    warranty_end DATE,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_inventory_updated_at
BEFORE UPDATE ON inventory
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_inventory_device_type ON inventory(device_type);
CREATE INDEX IF NOT EXISTS idx_inventory_site ON inventory(site);
