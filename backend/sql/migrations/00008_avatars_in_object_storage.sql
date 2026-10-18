-- +goose Up
-- Custom avatars move from a BYTEA table to the object store (Supabase Storage,
-- behind internal/storage). users keeps the public URL and the object path; the
-- path is what gets deleted when the avatar is replaced. Existing stored images
-- are dropped (development data only): those users fall back to "no avatar".
ALTER TABLE users
    ADD COLUMN avatar_url  TEXT,
    ADD COLUMN avatar_path TEXT,
    DROP CONSTRAINT users_avatar_one_kind,
    DROP COLUMN avatar_updated_at;

DROP TABLE user_avatars;

ALTER TABLE users
    ADD CONSTRAINT users_avatar_one_kind      CHECK (avatar_default IS NULL OR avatar_url IS NULL),
    ADD CONSTRAINT users_avatar_url_has_path  CHECK ((avatar_url IS NULL) = (avatar_path IS NULL));

-- +goose Down
ALTER TABLE users
    DROP CONSTRAINT users_avatar_url_has_path,
    DROP CONSTRAINT users_avatar_one_kind,
    DROP COLUMN avatar_path,
    DROP COLUMN avatar_url,
    ADD COLUMN avatar_updated_at TIMESTAMPTZ,
    ADD CONSTRAINT users_avatar_one_kind CHECK (avatar_default IS NULL OR avatar_updated_at IS NULL);

CREATE TABLE user_avatars (
    user_id    UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    data       BYTEA       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_avatars_size CHECK (octet_length(data) <= 1048576)
);
