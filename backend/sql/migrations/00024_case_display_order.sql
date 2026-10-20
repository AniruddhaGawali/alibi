-- +goose Up
-- The admin-chosen order of the cases, used by the admin case list and the players'
-- dashboard (lowest first). Existing cases keep today's order, newest first.
ALTER TABLE stories ADD COLUMN display_order INTEGER NOT NULL DEFAULT 0;

UPDATE stories s SET display_order = o.n
FROM (SELECT id, (row_number() OVER (ORDER BY created_at DESC, id))::int AS n FROM stories) o
WHERE s.id = o.id;

CREATE INDEX stories_display_order_idx ON stories (display_order, created_at DESC, id);

-- +goose Down
DROP INDEX stories_display_order_idx;
ALTER TABLE stories DROP COLUMN display_order;
