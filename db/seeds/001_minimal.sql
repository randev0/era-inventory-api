-- Minimal seed data for integration tests
-- This provides the basic lookup data needed to run tests

-- Insert test organization
INSERT INTO organizations (id, name, slug, created_at, updated_at) 
VALUES (1, 'Test Organization', 'test-org', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test user roles
INSERT INTO user_roles (id, name, description, created_at, updated_at)
VALUES 
    (1, 'org_admin', 'Organization Administrator', NOW(), NOW()),
    (2, 'project_admin', 'Project Administrator', NOW(), NOW()),
    (3, 'viewer', 'Read-only access', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test user
INSERT INTO users (id, email, name, organization_id, created_at, updated_at)
VALUES (1, 'test@example.com', 'Test User', 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test site
INSERT INTO sites (id, name, address, organization_id, created_at, updated_at)
VALUES (1, 'Test Site', '123 Test Street, Test City', 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test vendor
INSERT INTO vendors (id, name, contact_email, organization_id, created_at, updated_at)
VALUES (1, 'Test Vendor', 'vendor@example.com', 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test project
INSERT INTO projects (id, name, description, organization_id, created_at, updated_at)
VALUES (1, 'Test Project', 'A test project for integration tests', 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test item
INSERT INTO items (id, name, asset_tag, description, site_id, project_id, vendor_id, organization_id, created_at, updated_at)
VALUES (1, 'Test Item', 'TAG-001', 'A test inventory item', 1, 1, 1, 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Set RLS context for test user
-- This ensures the test user can access the test data
SELECT set_config('app.organization_id', '1', false);
SELECT set_config('app.user_id', '1', false);
