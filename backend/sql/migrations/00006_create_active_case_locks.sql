-- +goose Up
-- One row per user who is inside an unfinished case. The primary key is the
-- lock: a user cannot start a second case until the row is deleted.
CREATE TABLE active_case_locks (
    user_id   UUID        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    case_id   UUID        NOT NULL,
    story_id  UUID        NOT NULL REFERENCES stories (id),
    locked_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT active_case_locks_case_id_key UNIQUE (case_id)
);

-- +goose Down
DROP TABLE active_case_locks;
