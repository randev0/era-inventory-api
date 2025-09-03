-- 0010_assets_subtypes.sql - Optional subtypes for structured querying

-- Switch subtype (optional join; structured querying)
CREATE TABLE IF NOT EXISTS asset_switches (
  asset_id     BIGINT PRIMARY KEY REFERENCES assets(id) ON DELETE CASCADE,
  ports_total  INT,
  poe          BOOL,
  uplink_info  TEXT,
  firmware     TEXT
);

-- VLAN subtype (enforce unique vlan_id per site)
CREATE TABLE IF NOT EXISTS asset_vlans (
  asset_id   BIGINT PRIMARY KEY REFERENCES assets(id) ON DELETE CASCADE,
  vlan_id    INT NOT NULL,
  subnet     CIDR,
  gateway    INET,
  purpose    TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_vlan_per_site ON asset_vlans(
  vlan_id,
  (SELECT site_id FROM assets WHERE id = asset_vlans.asset_id),
  (SELECT org_id  FROM assets WHERE id = asset_vlans.asset_id)
);
