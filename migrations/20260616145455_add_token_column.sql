-- +goose Up
ALTER TABLE messages
    ADD COLUMN token_count INTEGER;

-- +goose Down
ALTER TABLE messages
    DROP COLUMN token_count;
