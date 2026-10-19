-- +goose Up
-- Background music per case (P5.3). The tracks are static files in the bucket at
-- music/<track>.ogg; this column holds the track key, '' for the main theme. The
-- API allow-lists the keys (admin.MusicTracks); the CHECK only keeps the value a
-- safe file name, so adding a track needs no migration.
ALTER TABLE stories
    ADD COLUMN music_track TEXT NOT NULL DEFAULT '',
    ADD CONSTRAINT stories_music_track_format CHECK (music_track ~ '^[a-z0-9-]{0,40}$');

-- +goose Down
ALTER TABLE stories
    DROP CONSTRAINT stories_music_track_format,
    DROP COLUMN music_track;
