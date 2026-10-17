-- +goose Up
-- Custom avatar images live in their own table so that "SELECT * FROM users"
-- never drags image bytes along. data is the processed 256x256 JPEG.
CREATE TABLE user_avatars (
    user_id    UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    data       BYTEA       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE user_avatars;
