-- +goose Up
ALTER TABLE messages
    ADD COLUMN tool_call_id TEXT;

-- +goose Down
ALTER TABLE messages
    DROP COLUMN tool_call_id;
