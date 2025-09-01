-- 002_default_users.sql - Default users for multi-tenant system

-- Ensure main tenant exists
INSERT INTO organizations (id, name) VALUES (1, 'Main Tenant') 
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

-- Create sample client organization
INSERT INTO organizations (name) VALUES ('Client Company A') ON CONFLICT (name) DO NOTHING;

-- Hash for password "Password123!" (bcrypt cost 10)
-- NOTE: In production, you should generate fresh hashes and provide secure default passwords

-- Main tenant super admin (password: Password123!)
INSERT INTO users (email, password_hash, first_name, last_name, org_id, roles) VALUES
('superadmin@maintenant.com', '$2a$10$qWPDkdfc22BJTfQqaMU83eSimkI5NFmGUrYSeFi1bDjEdOklPqvTu', 'Super', 'Admin', 1, '{"org_admin"}')
ON CONFLICT (email) DO UPDATE SET 
    password_hash = EXCLUDED.password_hash,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    org_id = EXCLUDED.org_id,
    roles = EXCLUDED.roles;

-- Sample client tenant admin (password: Password123!)
-- Note: We need to get the org_id for 'Client Company A' first
DO $$
DECLARE
    client_org_id BIGINT;
BEGIN
    SELECT id INTO client_org_id FROM organizations WHERE name = 'Client Company A';
    
    IF client_org_id IS NOT NULL THEN
        INSERT INTO users (email, password_hash, first_name, last_name, org_id, roles) VALUES
        ('admin@clienta.com', '$2a$10$qWPDkdfc22BJTfQqaMU83eSimkI5NFmGUrYSeFi1bDjEdOklPqvTu', 'Client', 'Admin', client_org_id, '{"org_admin"}')
        ON CONFLICT (email) DO UPDATE SET 
            password_hash = EXCLUDED.password_hash,
            first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name,
            org_id = EXCLUDED.org_id,
            roles = EXCLUDED.roles;
            
        -- Additional sample users for client org
        INSERT INTO users (email, password_hash, first_name, last_name, org_id, roles) VALUES
        ('manager@clienta.com', '$2a$10$qWPDkdfc22BJTfQqaMU83eSimkI5NFmGUrYSeFi1bDjEdOklPqvTu', 'Project', 'Manager', client_org_id, '{"project_admin"}'),
        ('viewer@clienta.com', '$2a$10$qWPDkdfc22BJTfQqaMU83eSimkI5NFmGUrYSeFi1bDjEdOklPqvTu', 'Data', 'Viewer', client_org_id, '{"viewer"}')
        ON CONFLICT (email) DO UPDATE SET 
            password_hash = EXCLUDED.password_hash,
            first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name,
            org_id = EXCLUDED.org_id,
            roles = EXCLUDED.roles;
    END IF;
END$$;

-- Update the sequence to ensure proper ID generation
SELECT setval('organizations_id_seq', (SELECT MAX(id) FROM organizations));
SELECT setval('users_id_seq', (SELECT MAX(id) FROM users));
