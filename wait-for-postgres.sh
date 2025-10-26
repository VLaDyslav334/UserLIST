#!/bin/bash
set -e

host="$1"
shift

until pg_isready -h "$host" -U "$POSTGRES_USER" --quiet; do
  echo "Waiting for PostgreSQL to be ready..."
  sleep 2
done

echo "PostgreSQL is ready - executing command"
exec "$@"