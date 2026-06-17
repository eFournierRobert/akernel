-- +goose Up
ALTER TABLE messages
    ADD COLUMN token_count INTEGER;

ALTER TABLE conversations
    ADD COLUMN last_summary INTEGER REFERENCES messages(id);

-- +goose Down
ALTER TABLE messages
    DROP COLUMN token_count;

ALTER TABLE conversations
    DROP COLUMN last_summary;
