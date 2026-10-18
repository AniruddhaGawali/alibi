-- +goose Up
-- The detective's own notes for a case. Private to the user. A note may be
-- about one suspect; if that suspect row is ever removed the note stays, untied.
CREATE TABLE case_notes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id)    ON DELETE CASCADE,
    story_id   UUID        NOT NULL REFERENCES stories (id)  ON DELETE CASCADE,
    suspect_id UUID                 REFERENCES suspects (id) ON DELETE SET NULL,
    body       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT case_notes_body_length CHECK (char_length(body) BETWEEN 1 AND 1000)
);

CREATE INDEX case_notes_user_story_idx ON case_notes (user_id, story_id, created_at DESC);

-- +goose Down
DROP TABLE case_notes;
