-- 0012_rls_assets_and_subtypes.sql - RLS policies for asset tables

ALTER TABLE assets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_assets ON assets
USING (
    current_setting('app.current_org_id')::bigint = 1 OR
    org_id = current_setting('app.current_org_id')::bigint
);

ALTER TABLE asset_switches ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_asset_switches ON asset_switches
USING (EXISTS (SELECT 1 FROM assets a
               WHERE a.id = asset_switches.asset_id
                 AND a.org_id = current_setting('app.current_org_id')::bigint));

ALTER TABLE asset_vlans ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_asset_vlans ON asset_vlans
USING (EXISTS (SELECT 1 FROM assets a
               WHERE a.id = asset_vlans.asset_id
                 AND a.org_id = current_setting('app.current_org_id')::bigint));

ALTER TABLE site_asset_categories ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_site_asset_categories ON site_asset_categories
USING (
    current_setting('app.current_org_id')::bigint = 1 OR
    org_id = current_setting('app.current_org_id')::bigint
);
