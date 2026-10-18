-- +goose Up
-- Phase 4.1: interrogations go through OpenRouter.
--
-- Each case picks its own OpenRouter model; new cases get the default Gemma model.
-- The format check keeps out routing suffixes that switch on tools, such as
-- ":online" (web search); the API validates the same rule before saving.
ALTER TABLE stories ADD COLUMN llm_model TEXT NOT NULL DEFAULT 'google/gemma-4-31b-it';
ALTER TABLE stories ADD CONSTRAINT stories_llm_model_format
    CHECK (llm_model ~ '^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*(:(free|nitro|floor))?$');

-- What the model needs to play a suspect. Server only: these columns are never
-- selected by a query that feeds a client response (hidden_truth gives the case away).
ALTER TABLE suspects
    ADD COLUMN relationship_to_victim TEXT NOT NULL DEFAULT '',  -- e.g. "his wife of twenty years"
    ADD COLUMN persona                TEXT NOT NULL DEFAULT '',  -- public personality; other suspects know it
    ADD COLUMN public_alibi           TEXT NOT NULL DEFAULT '',  -- the story they tell
    ADD COLUMN hidden_truth           TEXT NOT NULL DEFAULT '',  -- what really happened, never volunteered
    ADD COLUMN confession_triggers    TEXT NOT NULL DEFAULT '';  -- what makes them crack or admit the truth

-- Admins can create and edit cases (POST/PATCH /api/admin/cases).
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN is_admin;
ALTER TABLE suspects
    DROP COLUMN confession_triggers,
    DROP COLUMN hidden_truth,
    DROP COLUMN public_alibi,
    DROP COLUMN persona,
    DROP COLUMN relationship_to_victim;
ALTER TABLE stories DROP CONSTRAINT stories_llm_model_format;
ALTER TABLE stories DROP COLUMN llm_model;
