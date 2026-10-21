-- +goose Up
-- Detective tips for a case, written by the admin and shown under the chat box to
-- players in investigator mode (one at random). They go to every player in the
-- briefing, so they are nudges, never secrets. Empty = the frontend's generic tips.
ALTER TABLE stories
    ADD COLUMN chat_tips TEXT[] NOT NULL DEFAULT '{}',
    ADD CONSTRAINT stories_chat_tips_max CHECK (cardinality(chat_tips) <= 5);

-- +goose Down
ALTER TABLE stories
    DROP CONSTRAINT stories_chat_tips_max,
    DROP COLUMN chat_tips;
