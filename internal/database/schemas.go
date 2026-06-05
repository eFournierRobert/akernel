package database

import "time"

type Message struct {
	ID             int       `db:"id"`
	Role           string    `db:"role"`
	Content        string    `db:"content"`
	DateTime       time.Time `db:"datetime"`
	ConversationId int       `db:"conversation_id"`
}

type Conversation struct {
	ID    int    `db:"id"`
	Title string `db:"title"`
}
