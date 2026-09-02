-- +goose Up
ALTER TABLE messages
    ADD COLUMN uuid TEXT;

ALTER TABLE conversations
    ADD COLUMN uuid TEXT;

-- +goose Down
ALTER TABLE messages
    DROP COLUMN uuid;

ALTER TABLE conversations
    DROP COLUMN uuid;
