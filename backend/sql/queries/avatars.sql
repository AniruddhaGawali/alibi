-- name: SetDefaultAvatar :one
-- Choosing a default avatar clears any custom image. Returns the old custom
-- image's object path (NULL if there was none) so it can be deleted from storage.
UPDATE users u
SET avatar_default = @avatar_default, avatar_url = NULL, avatar_path = NULL
FROM (SELECT p.id, p.avatar_path FROM users p WHERE p.id = @id FOR UPDATE) old
WHERE u.id = old.id
RETURNING old.avatar_path AS old_path;

-- name: SetCustomAvatar :one
-- Points the user at an uploaded image. Returns the previous custom image's
-- object path (NULL if there was none) so it can be deleted from storage.
UPDATE users u
SET avatar_default = NULL, avatar_url = @avatar_url::text, avatar_path = @avatar_path::text
FROM (SELECT p.id, p.avatar_path FROM users p WHERE p.id = @id FOR UPDATE) old
WHERE u.id = old.id
RETURNING old.avatar_path AS old_path;
