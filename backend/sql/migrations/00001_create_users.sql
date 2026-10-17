-- +goose Up
CREATE TABLE users (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username          TEXT        NOT NULL,
    email             TEXT        NOT NULL,
    password_hash     TEXT        NOT NULL,          -- bcrypt hash, never the password
    avatar_default    SMALLINT,                      -- 1..4 when a default avatar is chosen
    avatar_updated_at TIMESTAMPTZ,                   -- set iff a custom avatar exists (see user_avatars)
    trophies          INTEGER     NOT NULL DEFAULT 0,
    token_balance     INTEGER     NOT NULL DEFAULT 0, -- source of truth; Redis only caches it
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_avatar_default_range    CHECK (avatar_default IS NULL OR avatar_default BETWEEN 1 AND 4),
    CONSTRAINT users_avatar_one_kind         CHECK (avatar_default IS NULL OR avatar_updated_at IS NULL),
    CONSTRAINT users_token_balance_non_negative CHECK (token_balance >= 0)
);

-- Case-insensitive uniqueness. The Go store maps violations of these two
-- constraint names to "email taken" / "username taken" (409).
CREATE UNIQUE INDEX users_email_key    ON users (lower(email));
CREATE UNIQUE INDEX users_username_key ON users (lower(username));

-- +goose Down
DROP TABLE users;
