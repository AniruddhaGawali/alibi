-- name: ListStories :many
-- Metadata for the dashboard cards, newest first. The same for every player (so it can be
-- cached and shared); how each player's own attempts ended is ListUserOutcomes.
SELECT s.id, s.title, s.synopsis, s.thumbnail_url, s.difficulty_level, s.reward_trophies
FROM stories s
WHERE s.is_published
ORDER BY s.created_at DESC, s.id;

-- name: ListUserOutcomes :many
-- How the caller's closed cases ended: story id and whether they solved it.
SELECT story_id, correct FROM case_results WHERE user_id = @user_id;

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
-- The case the user is locked into, with its story's id, title and music track
-- ('' = the main theme). A lock on a story that was unpublished since does not count.
SELECT l.case_id, l.story_id, s.title, s.music_track
FROM active_case_locks l
JOIN stories s ON s.id = l.story_id AND s.is_published
WHERE l.user_id = @user_id;

-- name: UsernamesByIDs :many
-- Display names for the ids the Redis leaderboard returned.
SELECT id, username FROM users WHERE id = ANY(@ids::uuid[]);
