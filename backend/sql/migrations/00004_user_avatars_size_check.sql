-- +goose Up
-- The 1 MB limit is enforced in Go; this keeps a bug or a future code path from
-- storing oversized blobs.
ALTER TABLE user_avatars
    ADD CONSTRAINT user_avatars_size CHECK (octet_length(data) <= 1048576);

-- +goose Down
ALTER TABLE user_avatars DROP CONSTRAINT user_avatars_size;
