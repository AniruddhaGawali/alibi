-- +goose Up
-- Progressive hints. Admins write up to 3 per case (in order, easiest first). A player
-- reveals them one at a time during a play-through; hint n costs n trophies, taken
-- off the reward only if they win (1 hint = 1, 2 = 3, 3 = 6 in total).
ALTER TABLE stories
    ADD COLUMN hints TEXT[] NOT NULL DEFAULT '{}',  -- server only: a hint is sent only once revealed
    ADD CONSTRAINT stories_hints_max CHECK (cardinality(hints) <= 3);

ALTER TABLE active_case_locks
    ADD COLUMN hints_used INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT active_case_locks_hints_used_range CHECK (hints_used BETWEEN 0 AND 3);

-- What the hints cost on a win, so the closed case can show it after a reload.
ALTER TABLE case_results
    ADD COLUMN hint_penalty INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT case_results_hint_penalty_non_negative CHECK (hint_penalty >= 0);

-- +goose Down
ALTER TABLE case_results
    DROP CONSTRAINT case_results_hint_penalty_non_negative,
    DROP COLUMN hint_penalty;
ALTER TABLE active_case_locks
    DROP CONSTRAINT active_case_locks_hints_used_range,
    DROP COLUMN hints_used;
ALTER TABLE stories
    DROP CONSTRAINT stories_hints_max,
    DROP COLUMN hints;
