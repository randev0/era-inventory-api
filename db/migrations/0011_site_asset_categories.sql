-- 0011_site_asset_categories.sql - Dynamic site categories with triggers

CREATE TABLE IF NOT EXISTS site_asset_categories (
  org_id     BIGINT NOT NULL,
  site_id    BIGINT NOT NULL,
  asset_type TEXT NOT NULL,
  asset_count INT NOT NULL DEFAULT 0,
  PRIMARY KEY (org_id, site_id, asset_type)
);

CREATE OR REPLACE FUNCTION refresh_site_asset_categories(p_org BIGINT, p_site BIGINT, p_type TEXT)
RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO site_asset_categories (org_id, site_id, asset_type, asset_count)
  SELECT p_org, p_site, p_type, COUNT(*)
  FROM assets
  WHERE org_id = p_org AND site_id = p_site AND asset_type = p_type
  ON CONFLICT (org_id, site_id, asset_type)
  DO UPDATE SET asset_count = EXCLUDED.asset_count;
END $$;

CREATE OR REPLACE FUNCTION trg_assets_refresh_counts()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE old_org BIGINT; old_site BIGINT; old_type TEXT;
        new_org BIGINT; new_site BIGINT; new_type TEXT;
BEGIN
  IF (TG_OP = 'INSERT') THEN
    new_org := NEW.org_id; new_site := NEW.site_id; new_type := NEW.asset_type;
    PERFORM refresh_site_asset_categories(new_org, new_site, new_type); RETURN NEW;
  ELSIF (TG_OP = 'UPDATE') THEN
    old_org := OLD.org_id; old_site := OLD.site_id; old_type := OLD.asset_type;
    new_org := NEW.org_id; new_site := NEW.site_id; new_type := NEW.asset_type;
    IF (old_org,old_site,old_type) IS DISTINCT FROM (new_org,new_site,new_type)
    THEN PERFORM refresh_site_asset_categories(old_org, old_site, old_type); END IF;
    PERFORM refresh_site_asset_categories(new_org, new_site, new_type); RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
    old_org := OLD.org_id; old_site := OLD.site_id; old_type := OLD.asset_type;
    PERFORM refresh_site_asset_categories(old_org, old_site, old_type); RETURN OLD;
  END IF; RETURN NULL;
END $$;

DROP TRIGGER IF EXISTS assets_refresh_counts ON assets;
CREATE TRIGGER assets_refresh_counts
AFTER INSERT OR UPDATE OR DELETE ON assets
FOR EACH ROW EXECUTE FUNCTION trg_assets_refresh_counts();
