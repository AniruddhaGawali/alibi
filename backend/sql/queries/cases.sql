-- name: GetCaseBriefing :one
-- A published case's briefing text. Unpublished cases do not exist for players.
SELECT id, title, synopsis, main_plot, difficulty_level, reward_trophies
FROM stories
WHERE id = @id AND is_published;

-- name: ListCaseScenes :many
SELECT id, scene_name, image_url, display_order
FROM case_scenes
WHERE case_id = @case_id
ORDER BY display_order, id;

-- name: ListCaseSuspects :many
-- Columns are listed explicitly: is_culprit (and anything else internal) must
-- never be selected by a query that feeds a client response.
SELECT id, name, role, avatar_url, initial_testimony
FROM suspects
WHERE case_id = @case_id
ORDER BY display_order, name;

-- name: GetUserLock :one
-- The user's active case lock: the play-through id and the story it is for.
SELECT case_id, story_id FROM active_case_locks WHERE user_id = @user_id;

-- name: CaseIsPublished :one
SELECT EXISTS (SELECT 1 FROM stories WHERE id = @id AND is_published) AS published;
