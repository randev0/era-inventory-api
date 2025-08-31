-- Enable pg_trgm extension for trigram operations
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Add trigram indexes for fast ILIKE searches on name fields
CREATE INDEX IF NOT EXISTS idx_items_name_trgm    ON inventory USING GIN (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_projects_name_trgm ON projects  USING GIN (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_vendors_name_trgm  ON vendors   USING GIN (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_sites_name_trgm    ON sites     USING GIN (name gin_trgm_ops);
