-- +goose Up
-- A note can be tied to one interrogation log entry. The entry lives in MongoDB
-- (chat_messages), so this is its event id, checked by the API when the note is
-- saved, not a foreign key.
ALTER TABLE case_notes ADD COLUMN chat_message_id TEXT;
ALTER TABLE case_notes ADD CONSTRAINT case_notes_chat_message_id_length
    CHECK (chat_message_id IS NULL OR char_length(chat_message_id) BETWEEN 1 AND 64);

-- +goose Down
ALTER TABLE case_notes DROP CONSTRAINT case_notes_chat_message_id_length;
ALTER TABLE case_notes DROP COLUMN chat_message_id;
