#!/usr/bin/env bash
set -euo pipefail
echo "=== CI Doctor ==="
: "${TEST_DATABASE_URL:?Missing TEST_DATABASE_URL}"
HOST=$(echo "$TEST_DATABASE_URL" | sed -E 's#^.+://[^@]+@([^:/]+).*$#\1#')
PORT=$(echo "$TEST_DATABASE_URL" | sed -E 's#^.+://[^@]+@[^:/]+:([0-9]+).*$#\1#' || true); PORT=${PORT:-5432}
echo "Checking DB at $HOST:$PORT ..."
for i in {1..30}; do pg_isready -h "$HOST" -p "$PORT" >/dev/null 2>&1 && break || sleep 1; done
psql "$TEST_DATABASE_URL" -c "SELECT current_user, current_database();"
psql "$TEST_DATABASE_URL" -c "CREATE TABLE IF NOT EXISTS ci_probe (id serial primary key);"
echo "CI Doctor OK."
