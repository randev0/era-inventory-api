-- Add nullable FKs (safe if re-run)
ALTER TABLE inventory ADD COLUMN IF NOT EXISTS site_id    BIGINT;
ALTER TABLE inventory ADD COLUMN IF NOT EXISTS vendor_id  BIGINT;
ALTER TABLE inventory ADD COLUMN IF NOT EXISTS project_id BIGINT;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_inventory_site') THEN
    ALTER TABLE inventory
      ADD CONSTRAINT fk_inventory_site
      FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_inventory_vendor') THEN
    ALTER TABLE inventory
      ADD CONSTRAINT fk_inventory_vendor
      FOREIGN KEY (vendor_id) REFERENCES vendors(id) ON DELETE SET NULL;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_inventory_project') THEN
    ALTER TABLE inventory
      ADD CONSTRAINT fk_inventory_project
      FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL;
  END IF;
END$$;

CREATE INDEX IF NOT EXISTS idx_inventory_site_id    ON inventory(site_id);
CREATE INDEX IF NOT EXISTS idx_inventory_vendor_id  ON inventory(vendor_id);
CREATE INDEX IF NOT EXISTS idx_inventory_project_id ON inventory(project_id);
