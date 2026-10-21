-- name: GetCaseBriefing :one
-- A published case's briefing text. Unpublished cases do not exist for players.
-- hints_total only counts the hints; their text is never selected here. chat_tips are
-- public (shown to investigators under the chat box).
SELECT id, title, synopsis, main_plot, difficulty_level, reward_trophies, cardinality(hints)::int AS hints_total, chat_tips
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
SELECT id, name, role, avatar_url, initial_testimony, public_knowledge
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
-- the user already closed this case, or is already locked into a live case. A lock left on a story that was
-- unpublished since is taken over.
INSERT INTO active_case_locks (user_id, case_id, story_id)
SELECT @user_id::uuid, @case_id::uuid, s.id
FROM stories s
WHERE s.id = @story_id AND s.is_published
  AND NOT EXISTS (SELECT 1 FROM case_results r WHERE r.user_id = @user_id AND r.story_id = s.id)
ON CONFLICT (user_id) DO UPDATE
SET case_id = EXCLUDED.case_id, story_id = EXCLUDED.story_id, locked_at = now()
WHERE NOT EXISTS (
    SELECT 1 FROM stories old WHERE old.id = active_case_locks.story_id AND old.is_published
);

-- name: CaseIsClosed :one
SELECT EXISTS (SELECT 1 FROM case_results WHERE user_id = @user_id AND story_id = @story_id) AS closed;

-- name: GetCaseResult :one
-- The player's settled accusation with the closing reveal (resolution, culprit).
-- Only ever read for a case the player has closed, so the solution is public to them.
SELECT r.correct, r.trophies_change, r.trophies_after, r.hint_penalty,
       st.resolution, c.id AS culprit_id, c.name AS culprit_name, c.hidden_truth
FROM case_results r
JOIN stories st ON st.id = r.story_id
JOIN suspects c ON c.case_id = r.story_id AND c.is_culprit
WHERE r.user_id = @user_id AND r.story_id = @story_id;

-- name: GetUnlockedHints :one
-- The hints this player has already revealed in their current play-through of the story.
-- No row means they are not playing it.
SELECT l.hints_used, COALESCE(s.hints[1:l.hints_used], '{}')::text[] AS hints
FROM active_case_locks l
JOIN stories s ON s.id = l.story_id
WHERE l.user_id = @user_id AND l.story_id = @story_id;

-- name: GetHintState :one
-- How many hints the player has used and how many the case has. No row: not playing it.
SELECT l.hints_used, cardinality(s.hints)::int AS hints_total
FROM active_case_locks l
JOIN stories s ON s.id = l.story_id AND s.is_published
WHERE l.user_id = @user_id AND l.story_id = @story_id;

-- name: RevealNextHint :one
-- Reveals the next hint in one statement: the counter moves up only while hints remain,
-- so two quick clicks cannot reveal (or pay for) the same hint twice. hints_used in
-- RETURNING is the new count, which is also the 1-based position of the hint just revealed.
UPDATE active_case_locks AS l
SET hints_used = l.hints_used + 1
FROM stories s
WHERE l.user_id = @user_id AND l.story_id = @story_id
  AND s.id = l.story_id AND s.is_published
  AND l.hints_used < LEAST(3, cardinality(s.hints))
RETURNING l.hints_used, s.hints[l.hints_used]::text AS hint;
