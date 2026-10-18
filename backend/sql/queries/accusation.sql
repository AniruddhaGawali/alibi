-- name: ReleaseCaseLock :one
-- Deletes the caller's lock for this story and says which play-through it was.
-- No row means the player is not in this case; because the delete is the claim,
-- two simultaneous accusations cannot both be paid. hints_used prices the win.
DELETE FROM active_case_locks
WHERE user_id = @user_id AND story_id = @story_id
RETURNING case_id, hints_used;

-- name: GetAccusationCase :one
-- Server only: the payout rules and the closing text of a case.
SELECT reward_trophies, penalty_trophies, resolution
FROM stories
WHERE id = @id;

-- name: GetAccusedSuspect :one
-- Server only: is_culprit never reaches a client before the case is closed.
SELECT id, name, is_culprit
FROM suspects
WHERE id = @id AND case_id = @case_id;

-- name: GetCulprit :one
-- The true culprit, with their own account, for the closing reveal.
SELECT id, name, hidden_truth
FROM suspects
WHERE case_id = @case_id AND is_culprit;

-- name: ApplyAccusationResult :one
-- Adds @delta to the player's trophies (never below zero) and counts a solved
-- case. Returns the balance before and after, so the leaderboard can be moved by
-- exactly what changed.
WITH prev AS (
    SELECT users.id, users.trophies FROM users WHERE users.id = @user_id FOR UPDATE
)
UPDATE users u
SET trophies = GREATEST(prev.trophies + @delta::int, 0),
    cases_solved = u.cases_solved + @solved::int
FROM prev
WHERE u.id = prev.id
RETURNING prev.trophies AS trophies_before, u.trophies AS trophies_after;

-- name: InsertCaseResult :exec
-- Closes the case for the player. The primary key refuses a second result.
INSERT INTO case_results (user_id, story_id, suspect_id, correct, trophies_change, trophies_after, hint_penalty)
VALUES (@user_id, @story_id, @suspect_id, @correct, @trophies_change, @trophies_after, @hint_penalty);
