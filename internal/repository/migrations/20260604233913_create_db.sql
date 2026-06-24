-- +goose Up
CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT
);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    role TEXT,
    content TEXT,
    datetime INTEGER,
    conversation_id INTEGER,
    CONSTRAINT fk_converation_message
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE messages;
DROP TABLE conversations;
