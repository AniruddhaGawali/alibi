-- +goose Up
-- A 32-bit balance overflows after repeated grants ("integer out of range").
ALTER TABLE users ALTER COLUMN token_balance TYPE BIGINT;

-- +goose Down
ALTER TABLE users ALTER COLUMN token_balance TYPE INTEGER;
