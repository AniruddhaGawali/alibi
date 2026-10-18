-- +goose Up
CREATE TABLE stories (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title            TEXT        NOT NULL,
    synopsis         TEXT        NOT NULL,
    thumbnail_url    TEXT        NOT NULL DEFAULT '',
    difficulty_level TEXT        NOT NULL,
    reward_trophies  INTEGER     NOT NULL DEFAULT 0,
    reward_tokens    INTEGER     NOT NULL DEFAULT 0,
    is_published     BOOLEAN     NOT NULL DEFAULT true,   -- only published stories reach the dashboard
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT stories_difficulty_level_valid       CHECK (difficulty_level IN ('easy', 'medium', 'hard')),
    CONSTRAINT stories_reward_trophies_non_negative CHECK (reward_trophies >= 0),
    CONSTRAINT stories_reward_tokens_non_negative   CHECK (reward_tokens >= 0)
);

CREATE INDEX stories_published_created_idx ON stories (created_at DESC, id) WHERE is_published;

-- +goose Down
DROP TABLE stories;
