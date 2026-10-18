#!/bin/sh
# Loads the development test data into the docker-compose Postgres. The
# leaderboard is read straight from Postgres, so nothing else needs seeding.
set -eu
cd "$(dirname "$0")/../../.."

PSQL="docker compose exec -T postgres psql -U postgres -d alibi -v ON_ERROR_STOP=1"

# Image URLs point at the Supabase bucket when it is configured (make exports
# the repo .env), otherwise at the local copy in frontend/public.
ASSET_BASE=""
if [ -n "${SUPABASE_URL:-}" ]; then
	ASSET_BASE="${SUPABASE_URL%/}/storage/v1/object/public/${SUPABASE_BUCKET_NAME:-alibi-assets}"
fi

$PSQL -q -v asset_base="$ASSET_BASE" < backend/sql/seed/dev_seed.sql
echo "seeded: $($PSQL -tA -c 'SELECT count(*) FROM stories') stories, $($PSQL -tA -c 'SELECT count(*) FROM users WHERE trophies > 0') ranked users"
