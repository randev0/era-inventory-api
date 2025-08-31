-- 0006_rls.sql
ALTER TABLE sites     ENABLE ROW LEVEL SECURITY;
ALTER TABLE vendors   ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects  ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname='public' AND tablename='sites' AND policyname='org_isolation_sites') THEN
    CREATE POLICY org_isolation_sites ON sites
      USING (org_id = current_setting('app.current_org_id')::bigint);
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname='public' AND tablename='vendors' AND policyname='org_isolation_vendors') THEN
    CREATE POLICY org_isolation_vendors ON vendors
      USING (org_id = current_setting('app.current_org_id')::bigint);
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname='public' AND tablename='projects' AND policyname='org_isolation_projects') THEN
    CREATE POLICY org_isolation_projects ON projects
      USING (org_id = current_setting('app.current_org_id')::bigint);
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname='public' AND tablename='inventory' AND policyname='org_isolation_inventory') THEN
    CREATE POLICY org_isolation_inventory ON inventory
      USING (org_id = current_setting('app.current_org_id')::bigint);
  END IF;
END$$;
