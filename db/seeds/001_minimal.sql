-- Minimal seed data for integration tests
-- This provides the basic lookup data needed to run tests

-- Insert test organization
INSERT INTO organizations (id, name, created_at, updated_at) 
VALUES (1, 'Test Organization', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test site
INSERT INTO sites (id, name, code, org_id, created_at, updated_at)
VALUES (1, 'Test Site', 'TEST-SITE-001', 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test vendor
INSERT INTO vendors (id, name, contact, org_id, created_at, updated_at)
VALUES (1, 'Test Vendor', 'vendor@example.com', 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test project
INSERT INTO projects (id, name, code, org_id, created_at, updated_at)
VALUES (1, 'Test Project', 'TEST-PROJ-001', 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test inventory item
INSERT INTO inventory (id, asset_tag, name, manufacturer, model, device_type, site, org_id, created_at, updated_at)
VALUES (1, 'TAG-001', 'Test Item', 'Test Manufacturer', 'Test Model', 'Desktop', 'Test Site', 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Set RLS context for test user
-- This ensures the test user can access the test data
SELECT set_config('app.organization_id', '1', false);
