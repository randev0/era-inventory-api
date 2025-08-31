INSERT INTO organizations(name) VALUES ('Default Org')
ON CONFLICT DO NOTHING;

-- Use org_id=1 for demo rows
INSERT INTO sites(name, code, org_id)
SELECT 'HQ Campus','HQ-BLK-A',1 WHERE NOT EXISTS (SELECT 1 FROM sites WHERE name='HQ Campus' AND org_id=1);

INSERT INTO vendors(name, contact, org_id)
SELECT 'Tech Supplies','hello@vendor.my, +60 12-345 6789',1
WHERE NOT EXISTS (SELECT 1 FROM vendors WHERE name='Tech Supplies' AND org_id=1);

INSERT INTO projects(code, name, org_id)
SELECT 'GDN-ESS','ESS Revamp',1
WHERE NOT EXISTS (SELECT 1 FROM projects WHERE code='GDN-ESS' AND org_id=1);

INSERT INTO inventory(asset_tag, name, manufacturer, model, device_type, site, org_id, site_id, vendor_id, project_id)
SELECT 'DELL-001','Dell OptiPlex','Dell','OptiPlex 7090','Desktop','HQ Campus',1,s.id,v.id,p.id
FROM sites s, vendors v, projects p
WHERE s.name='HQ Campus' AND v.name='Tech Supplies' AND p.code='GDN-ESS'
  AND NOT EXISTS (
    SELECT 1 FROM inventory i WHERE i.asset_tag='DELL-001' AND i.org_id=1
  );
