-- +goose Up
-- Final accusation (Phase 5.1). Both values are set by admins when they add a case,
-- next to reward_trophies: what a wrong accusation costs, and the text that reveals
-- the truth of the crime once the case is closed. Trophies only: no token rewards.
ALTER TABLE stories
    ADD COLUMN penalty_trophies INTEGER NOT NULL DEFAULT 5,
    ADD COLUMN resolution       TEXT    NOT NULL DEFAULT '',  -- server only until the case is closed
    ADD CONSTRAINT stories_penalty_trophies_non_negative CHECK (penalty_trophies >= 0);

-- +goose Down
ALTER TABLE stories
    DROP CONSTRAINT stories_penalty_trophies_non_negative,
    DROP COLUMN resolution,
    DROP COLUMN penalty_trophies;
