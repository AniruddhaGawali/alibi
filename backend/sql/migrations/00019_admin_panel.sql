-- +goose Up
-- Phase 5.3: admin panel.
--
-- users.role replaces users.is_admin. The role also goes into the access token so
-- the frontend can route admins to /admin; the API still re-reads it from this
-- column on every admin request, so a demotion takes effect at once.
ALTER TABLE users
    ADD COLUMN role TEXT NOT NULL DEFAULT 'user',
    ADD CONSTRAINT users_role_valid CHECK (role IN ('user', 'admin'));
UPDATE users SET role = 'admin' WHERE is_admin;
ALTER TABLE users DROP COLUMN is_admin;

-- Global game settings, edited from the admin panel. Exactly one row (id is always
-- true); the API inserts it at startup from DAILY_TOKENS / TOKEN_RESET_TIME /
-- TOKEN_RESET_TIMEZONE when it is missing, and the admin panel owns it after that.
-- daily_tokens is capped at 10, the most a player can hold (users_token_balance_max).
CREATE TABLE app_settings (
    id                   BOOLEAN     PRIMARY KEY DEFAULT true,
    daily_tokens         INTEGER     NOT NULL,
    token_reset_time     TEXT        NOT NULL,  -- HH:MM, 24-hour clock
    token_reset_timezone TEXT        NOT NULL,  -- IANA zone the reset time is read in
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by           UUID        REFERENCES users (id) ON DELETE SET NULL,

    CONSTRAINT app_settings_single_row        CHECK (id),
    CONSTRAINT app_settings_daily_tokens_range CHECK (daily_tokens BETWEEN 1 AND 10),
    CONSTRAINT app_settings_reset_time_format  CHECK (token_reset_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    CONSTRAINT app_settings_timezone_not_empty CHECK (token_reset_timezone <> '')
);

-- +goose Down
DROP TABLE app_settings;
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;
UPDATE users SET is_admin = true WHERE role = 'admin';
ALTER TABLE users
    DROP CONSTRAINT users_role_valid,
    DROP COLUMN role;
