-- +goose Up
-- +goose StatementBegin
-- Server-side gate for the confession: the hidden truth only goes into a suspect's
-- prompt once the detective's question matches one of these groups. Each element is
-- a group of terms joined by "+", all of which must start a word of the question
-- (e.g. 'last+study' matches "Who was last in the study?"). Empty means no gate:
-- the suspect's secrets are always in the prompt (the behaviour before this column).
ALTER TABLE suspects
    ADD COLUMN confession_keywords TEXT[] NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE suspects
    DROP COLUMN confession_keywords;
-- +goose StatementEnd
