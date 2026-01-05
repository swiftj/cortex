#!/bin/sh
set -e

# Wait for PostgreSQL to be ready
until pg_isready -h "${POSTGRES_HOST:-postgres}" -p "${POSTGRES_PORT:-5432}" -U "${POSTGRES_USER:-cortex}" -d "${POSTGRES_DB:-cortex}" > /dev/null 2>&1; do
  echo "Waiting for PostgreSQL to be ready..."
  sleep 2
done

echo "PostgreSQL is ready, starting Cortex..."

# Run the cortex binary
exec /usr/local/bin/cortex "$@"
