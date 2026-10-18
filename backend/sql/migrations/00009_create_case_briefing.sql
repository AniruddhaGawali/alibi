-- +goose Up
-- Phase 3.1: what the briefing page shows. "Case" = a row of stories (the
-- dashboard calls it a story). Scenes and suspects hang off it.
ALTER TABLE stories ADD COLUMN main_plot TEXT NOT NULL DEFAULT '';

CREATE TABLE case_scenes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id       UUID    NOT NULL REFERENCES stories (id) ON DELETE CASCADE,
    scene_name    TEXT    NOT NULL,
    image_url     TEXT    NOT NULL,  -- public URL from the storage provider
    display_order INTEGER NOT NULL DEFAULT 0,

    CONSTRAINT case_scenes_order_key UNIQUE (case_id, display_order)
);

CREATE TABLE suspects (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id           UUID    NOT NULL REFERENCES stories (id) ON DELETE CASCADE,
    name              TEXT    NOT NULL,
    role              TEXT    NOT NULL,
    avatar_url        TEXT    NOT NULL DEFAULT '',
    initial_testimony TEXT    NOT NULL,   -- shown in the briefing tooltip and modal
    is_culprit        BOOLEAN NOT NULL DEFAULT false,  -- server only: never selected by a client-facing query
    display_order     INTEGER NOT NULL DEFAULT 0,

    CONSTRAINT suspects_name_key UNIQUE (case_id, name)
);

-- At most one culprit per case.
CREATE UNIQUE INDEX suspects_one_culprit_idx ON suspects (case_id) WHERE is_culprit;

-- +goose Down
DROP TABLE suspects;
DROP TABLE case_scenes;
ALTER TABLE stories DROP COLUMN main_plot;
