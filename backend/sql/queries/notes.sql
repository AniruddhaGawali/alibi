-- name: ListCaseNotes :many
-- The user's notes for one case, newest first.
SELECT id, suspect_id, chat_message_id, body, created_at
FROM case_notes
WHERE user_id = @user_id AND story_id = @story_id
ORDER BY created_at DESC, id;

-- name: CreateCaseNote :one
-- Adds a note unless the user already has @max_notes for this case (no row is
-- returned then). The suspect, if any, must already be checked with SuspectInCase,
-- and the log entry, if any, against the chat history.
INSERT INTO case_notes (user_id, story_id, suspect_id, chat_message_id, body)
SELECT @user_id::uuid, @story_id::uuid, sqlc.narg('suspect_id')::uuid, sqlc.narg('chat_message_id')::text, @body::text
WHERE (SELECT count(*) FROM case_notes WHERE user_id = @user_id AND story_id = @story_id) < @max_notes::int
RETURNING id, suspect_id, chat_message_id, body, created_at;

-- name: SuspectInCase :one
SELECT EXISTS (SELECT 1 FROM suspects WHERE id = @id AND case_id = @case_id) AS found;

-- name: DeleteCaseNote :execrows
-- Only the owner can delete a note.
DELETE FROM case_notes WHERE id = @id AND user_id = @user_id;
