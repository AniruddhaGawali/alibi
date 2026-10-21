-- +goose Up
-- The most tokens a player can hold becomes a setting (app_settings.max_tokens,
-- edited in the admin panel) instead of a fixed 10. The daily allowance can never be
-- above it. users_token_balance_max stays as an absolute ceiling (user.TokenCeiling).
ALTER TABLE app_settings
    ADD COLUMN max_tokens INTEGER NOT NULL DEFAULT 10,
    ADD CONSTRAINT app_settings_max_tokens_range CHECK (max_tokens BETWEEN 1 AND 100),
    DROP CONSTRAINT app_settings_daily_tokens_range,
    ADD CONSTRAINT app_settings_daily_tokens_range CHECK (daily_tokens BETWEEN 1 AND 100),
    ADD CONSTRAINT app_settings_daily_within_max CHECK (daily_tokens <= max_tokens);

ALTER TABLE users
    DROP CONSTRAINT users_token_balance_max,
    ADD CONSTRAINT users_token_balance_max CHECK (token_balance <= 100);

-- +goose Down
UPDATE users SET token_balance = 10 WHERE token_balance > 10;
ALTER TABLE users
    DROP CONSTRAINT users_token_balance_max,
    ADD CONSTRAINT users_token_balance_max CHECK (token_balance <= 10);

UPDATE app_settings SET daily_tokens = LEAST(daily_tokens, 10);
ALTER TABLE app_settings
    DROP CONSTRAINT app_settings_daily_within_max,
    DROP CONSTRAINT app_settings_daily_tokens_range,
    ADD CONSTRAINT app_settings_daily_tokens_range CHECK (daily_tokens BETWEEN 1 AND 10),
    DROP CONSTRAINT app_settings_max_tokens_range,
    DROP COLUMN max_tokens;
