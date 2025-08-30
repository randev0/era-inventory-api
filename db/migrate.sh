#!/usr/bin/env sh
set -euo pipefail

echo "Waiting for Postgres at $PGHOST:$PGPORT/$PGDATABASE ..."
until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" >/dev/null 2>&1; do
  sleep 1
done
echo "Postgres is ready."

# Ledger table for idempotency
psql -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  id BIGSERIAL PRIMARY KEY,
  filename TEXT NOT NULL UNIQUE,
  checksum TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
SQL

apply_file () {
  f="$1"
  base="$(basename "$f")"
  sum="$(md5sum "$f" | awk '{print $1}')"
  if psql -tA -c "SELECT 1 FROM schema_migrations WHERE filename='${base}'" | grep -q 1; then
    echo ">> Skipping ${base} (already applied)"
    return
  fi
  echo ">> Applying ${base}"
  psql -v ON_ERROR_STOP=1 -f "$f"
  psql -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations(filename, checksum) VALUES ('${base}', '${sum}')"
}

# Apply in lexical order
for f in $(ls -1 db/migrations/*.sql 2>/dev/null | sort); do
  apply_file "$f"
done

echo "All migrations applied."
