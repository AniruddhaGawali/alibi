-- +goose Up
-- Sign-in providers linked to a user (Google today). The provider's stable subject id
-- identifies the person, never the email, which can change. Accounts created through
-- a provider have no password: users.password_hash holds the marker '!', which is not
-- a bcrypt hash, so no password can ever match it.
CREATE TABLE user_identities (
    provider   TEXT        NOT NULL,
    subject    TEXT        NOT NULL,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email      TEXT        NOT NULL,                 -- the provider's email when it was linked (informational)
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT user_identities_pkey PRIMARY KEY (provider, subject),
    CONSTRAINT user_identities_provider_valid CHECK (provider IN ('google'))
);

-- one identity per provider per user
CREATE UNIQUE INDEX user_identities_user_provider_key ON user_identities (user_id, provider);

-- +goose Down
DROP TABLE user_identities;
