-- +goose Up
-- A player never holds more than 10 question tokens, and winning a case pays in
-- trophies only. The cap is enforced in the wallet queries and, as a backstop,
-- by a CHECK; keep both in step with user.MaxTokenBalance.
UPDATE users SET token_balance = 10 WHERE token_balance > 10;
ALTER TABLE users ADD CONSTRAINT users_token_balance_max CHECK (token_balance <= 10);

ALTER TABLE stories DROP COLUMN reward_tokens;

-- +goose Down
ALTER TABLE stories ADD COLUMN reward_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE stories ADD CONSTRAINT stories_reward_tokens_non_negative CHECK (reward_tokens >= 0);
ALTER TABLE users DROP CONSTRAINT users_token_balance_max;
