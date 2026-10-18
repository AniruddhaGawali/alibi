-- +goose Up
ALTER TABLE users
    ADD COLUMN cases_solved INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT users_cases_solved_non_negative CHECK (cases_solved >= 0);

-- +goose Down
ALTER TABLE users
    DROP CONSTRAINT users_cases_solved_non_negative,
    DROP COLUMN cases_solved;
