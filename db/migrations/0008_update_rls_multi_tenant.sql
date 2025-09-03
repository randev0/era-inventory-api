-- 0008_update_rls_multi_tenant.sql - Update existing RLS policies for multi-tenant access

-- Update existing RLS policies to respect main tenant access
-- Main tenant (org_id = 1) can see ALL data, others see only their org data

-- Drop existing policies
DROP POLICY IF EXISTS org_isolation_sites ON sites;
DROP POLICY IF EXISTS org_isolation_vendors ON vendors;
DROP POLICY IF EXISTS org_isolation_projects ON projects;
DROP POLICY IF EXISTS org_isolation_inventory ON inventory;

-- Create new multi-tenant policies
CREATE POLICY tenant_isolation_sites ON sites 
    USING (
        current_setting('app.current_org_id')::bigint = 1 OR
        org_id = current_setting('app.current_org_id')::bigint
    );

CREATE POLICY tenant_isolation_vendors ON vendors 
    USING (
        current_setting('app.current_org_id')::bigint = 1 OR
        org_id = current_setting('app.current_org_id')::bigint
    );

CREATE POLICY tenant_isolation_projects ON projects 
    USING (
        current_setting('app.current_org_id')::bigint = 1 OR
        org_id = current_setting('app.current_org_id')::bigint
    );

CREATE POLICY tenant_isolation_inventory ON inventory 
    USING (
        current_setting('app.current_org_id')::bigint = 1 OR
        org_id = current_setting('app.current_org_id')::bigint
    );
