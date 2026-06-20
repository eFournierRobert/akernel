-- +goose Up
ALTER TABLE messages
    ADD COLUMN thinking TEXT;

-- +goose Down
ALTER TABLE messages
    DROP COLUMN thinking;
