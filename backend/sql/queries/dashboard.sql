-- name: ListStories :many
-- Metadata for the dashboard cards, newest first.
SELECT id, title, synopsis, thumbnail_url, difficulty_level, reward_trophies, reward_tokens
FROM stories
WHERE is_published
ORDER BY created_at DESC, id;

-- name: GetUserStats :one
SELECT trophies, cases_solved FROM users WHERE id = @id;

-- name: GetUsernames :many
-- Maps leaderboard member IDs to display names.
SELECT id, username FROM users WHERE id = ANY(@ids::uuid[]);

-- name: TopPlayers :many
-- Leaderboard fallback when the Redis sorted set is empty or unreachable. Players
-- without trophies are not ranked. Ties go to whoever joined first.
SELECT id, username, trophies
FROM users
WHERE trophies > 0
ORDER BY trophies DESC, created_at, id
LIMIT @max_rows::int;

-- name: GetActiveCase :one
-- The case the user is locked into, with its story's title for display.
SELECT l.case_id, s.title
FROM active_case_locks l
JOIN stories s ON s.id = l.story_id
WHERE l.user_id = @user_id;

-- name: LockUserToCase :execrows
-- Takes the lock. Zero rows means the user is already locked into a case.
INSERT INTO active_case_locks (user_id, case_id, story_id)
VALUES (@user_id, @case_id, @story_id)
ON CONFLICT (user_id) DO NOTHING;

-- name: ReleaseCaseLock :execrows
DELETE FROM active_case_locks WHERE user_id = @user_id AND case_id = @case_id;
