#!/bin/sh
# Loads the development test data into the docker-compose Postgres and mirrors
# the trophy totals into the Redis leaderboard (the API reads Redis first).
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
$PSQL -tA -F ' ' -c "SELECT trophies, id FROM users WHERE trophies > 0" |
	while read -r trophies id; do echo "ZADD leaderboard:trophies $trophies $id"; done |
	docker compose exec -T redis redis-cli > /dev/null
echo "seeded: $($PSQL -tA -c 'SELECT count(*) FROM stories') stories, $($PSQL -tA -c 'SELECT count(*) FROM users WHERE trophies > 0') ranked users"
