-- +goose Up
-- One row per player per case, written when the final accusation is settled. A case
-- is played once: the row closes it (the player cannot start it again) and lets the
-- case page show the verdict after a reload.
CREATE TABLE case_results (
    user_id        UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    story_id       UUID        NOT NULL REFERENCES stories (id) ON DELETE CASCADE,
    suspect_id     UUID        REFERENCES suspects (id) ON DELETE SET NULL,  -- who was accused
    correct        BOOLEAN     NOT NULL,
    trophies_change INTEGER    NOT NULL,
    trophies_after INTEGER     NOT NULL,
    resolved_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT case_results_pkey PRIMARY KEY (user_id, story_id)
);

-- +goose Down
DROP TABLE case_results;
