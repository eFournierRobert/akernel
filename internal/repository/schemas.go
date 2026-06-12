package repository

import "time"

// Message represents a single message stored in the database.
type Message struct {
	ID             int       `db:"id"`
	Role           string    `db:"role"`
	Content        string    `db:"content"`
	DateTime       time.Time `db:"datetime"`
	ConversationId int       `db:"conversation_id"`
}

// Conversation represents a conversation thread with a title.
type Conversation struct {
	ID    int    `db:"id"`
	Title string `db:"title"`
}
