-- +goose Up
-- Difficulty modes. An "investigator" sees each suspect's public knowledge (background
-- the town already knows) and chat tips; a "master_sleuth" plays with the initial
-- testimony alone. Everyone starts as an investigator and can switch at any time.
ALTER TABLE users
    ADD COLUMN player_mode TEXT NOT NULL DEFAULT 'investigator',
    ADD CONSTRAINT users_player_mode_valid CHECK (player_mode IN ('investigator', 'master_sleuth'));

-- Background shown to investigators on the case page. It is public, never a secret:
-- it reaches every player's browser, whatever their mode.
ALTER TABLE suspects
    ADD COLUMN public_knowledge TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE suspects DROP COLUMN public_knowledge;
ALTER TABLE users DROP COLUMN player_mode;
