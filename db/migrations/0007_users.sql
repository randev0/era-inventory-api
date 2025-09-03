-- 0007_users.sql - User management system with multi-tenant support

-- Create updated_at trigger function if not exists
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    org_id BIGINT NOT NULL REFERENCES organizations(id),
    roles TEXT[] NOT NULL DEFAULT '{"viewer"}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    last_login_at TIMESTAMPTZ
);

-- Indexes and constraints
CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_org_email ON users(org_id, email);

-- Enable RLS for multi-tenant access
ALTER TABLE users ENABLE ROW LEVEL SECURITY;

-- Drop existing policy if it exists
DROP POLICY IF EXISTS tenant_isolation_users ON users;

-- Policy: Main tenant (org_id = 1) sees ALL users, others see only their org
CREATE POLICY tenant_isolation_users ON users 
    USING (
        current_setting('app.current_org_id')::bigint = 1 OR  -- Main tenant sees all
        org_id = current_setting('app.current_org_id')::bigint -- Others see only their org
    );

-- Triggers
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Insert main tenant if not exists
INSERT INTO organizations (id, name) VALUES (1, 'Main Tenant') 
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;
