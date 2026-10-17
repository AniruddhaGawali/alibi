-- name: SetDefaultAvatar :execrows
-- Choosing a default avatar drops any stored custom image.
WITH removed AS (
    DELETE FROM user_avatars WHERE user_id = @id
)
UPDATE users
SET avatar_default = @avatar_default, avatar_updated_at = NULL
WHERE id = @id;

-- name: SetCustomAvatar :one
-- Stores the image and marks the user as having a custom avatar in one statement.
WITH saved AS (
    INSERT INTO user_avatars (user_id, data, updated_at)
    VALUES (@id, @data, now())
    ON CONFLICT (user_id) DO UPDATE SET data = EXCLUDED.data, updated_at = EXCLUDED.updated_at
    RETURNING updated_at
)
UPDATE users
SET avatar_default = NULL, avatar_updated_at = (SELECT updated_at FROM saved)
WHERE id = @id
RETURNING avatar_updated_at::timestamptz AS updated_at;

-- name: GetAvatar :one
SELECT data, updated_at FROM user_avatars WHERE user_id = @id;

-- name: GetAvatarUpdatedAt :one
-- Cheap revalidation lookup: no image bytes are read.
SELECT updated_at FROM user_avatars WHERE user_id = @id;
