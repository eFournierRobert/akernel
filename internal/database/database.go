package database

import (
	"github.com/adrg/xdg"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

// Database manages the persistence of conversations and messages.
type Database struct {
	db *sqlx.DB
}

// ConnectDB connects to SQLite, runs migrations, and sets the database path based on the isDev flag.
func ConnectDB(isDev bool) (*Database, error) {
	var dbPath string
	if !isDev {
		var err error
		dbPath, err = xdg.DataFile("akernel/akernel.db")
		if err != nil {
			return nil, err
		}
	} else {
		dbPath = "./database/database.db"
	}

	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	goose.SetBaseFS(nil)

	if err = goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}

	if err = goose.Up(db.DB, "migrations"); err != nil {
		return nil, err
	}

	database := Database{
		db: db,
	}
	return &database, nil
}

// InsertMessage saves a message to the database and creates a new conversation if necessary.
func (d *Database) InsertMessage(message *Message) error {
	_, err := d.db.NamedExec("INSERT INTO messages (role, content, conversation_id, datetime) VALUES (:role, :content, :conversation_id, :datetime)", message)
	return err
}

// SelectMessagesOfConvo retrieves all messages for a specific conversation, ordered by date.
func (d *Database) SelectMessagesOfConvo(conversationId int) ([]Message, error) {
	var messages []Message
	err := d.db.Select(&messages, "SELECT role, content FROM messages WHERE conversation_id = $1 ORDER BY datetime ASC", conversationId)
	if err != nil {
		return nil, err
	}
	return messages, nil
}

// SelectConversations retrieves a list of all available conversations.
func (d *Database) SelectConversations() ([]Conversation, error) {
	var conversations []Conversation
	err := d.db.Select(&conversations, "SELECT id, title FROM conversations")
	return conversations, err
}

// PopLastUserMessage removes the most recent message sent by the user in a specific conversation.
func (d *Database) PopLastUserMessage(conversationId int) error {
	_, err := d.db.Exec(
		"DELETE FROM messages WHERE id = (SELECT id FROM messages WHERE role = 'user' AND conversation_id = $1 ORDER BY datetime DESC LIMIT 1)",
		conversationId)
	return err
}

// EnsureConversation ensures a conversation exists in the database and populates its details.
func (d *Database) EnsureConversation(conversation *Conversation) error {
	_, err := d.db.NamedExec("INSERT OR IGNORE INTO conversations (title) VALUES (:title)", conversation)
	if err != nil {
		return err
	}

	err = d.db.Get(conversation, "SELECT id, title FROM conversations WHERE title = $1", conversation.Title)
	return err
}
