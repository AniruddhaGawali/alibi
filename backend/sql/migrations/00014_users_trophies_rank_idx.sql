-- +goose Up
-- Serves the dashboard leaderboard (ORDER BY trophies DESC, created_at, id LIMIT n)
-- with an index scan instead of a scan of the whole users table.
CREATE INDEX users_trophies_rank_idx ON users (trophies DESC, created_at, id) WHERE trophies > 0;

-- +goose Down
DROP INDEX users_trophies_rank_idx;
