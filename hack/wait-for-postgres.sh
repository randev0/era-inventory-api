#!/usr/bin/env sh
set -euo pipefail

# Default values
HOST=${PGHOST:-localhost}
PORT=${PGPORT:-5432}
USER=${PGUSER:-era}
DB=${PGDATABASE:-era_test}
TIMEOUT=${TIMEOUT:-30}

echo "Waiting for Postgres at $HOST:$PORT/$DB (timeout: ${TIMEOUT}s)..."

# Wait for postgres to be ready
for i in $(seq 1 $TIMEOUT); do
  if pg_isready -h "$HOST" -p "$PORT" -U "$USER" -d "$DB" >/dev/null 2>&1; then
    echo "Postgres is ready!"
    exit 0
  fi
  echo "Waiting... ($i/$TIMEOUT)"
  sleep 1
done

echo "Timeout waiting for Postgres"
exit 1
