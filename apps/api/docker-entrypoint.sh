#!/bin/sh
set -e

if [ "${MIGRATE_ON_START:-false}" = "true" ]; then
  migrate -dir /app/migrations
fi

if [ "${SEED_ON_START:-false}" = "true" ]; then
  seed
fi

exec api
