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
-- A lock on a story that was unpublished since is ignored, so it cannot trap the player.
SELECT l.case_id, l.story_id
FROM active_case_locks l
JOIN stories s ON s.id = l.story_id AND s.is_published
WHERE l.user_id = @user_id;

-- name: CaseIsPublished :one
SELECT EXISTS (SELECT 1 FROM stories WHERE id = @id AND is_published) AS published;

-- name: LockUserToCase :execrows
-- Locks the user into a published story in one statement, so there is no gap
-- between checking and inserting. Zero rows means the story is not published or
-- the user is already locked into a live case. A lock left on a story that was
-- unpublished since is taken over.
INSERT INTO active_case_locks (user_id, case_id, story_id)
SELECT @user_id::uuid, @case_id::uuid, s.id
FROM stories s
WHERE s.id = @story_id AND s.is_published
ON CONFLICT (user_id) DO UPDATE
SET case_id = EXCLUDED.case_id, story_id = EXCLUDED.story_id, locked_at = now()
WHERE NOT EXISTS (
    SELECT 1 FROM stories old WHERE old.id = active_case_locks.story_id AND old.is_published
);
