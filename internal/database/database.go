package database

import (
	"locassist/internal/ollama"

	"github.com/adrg/xdg"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

type Database struct {
	Db            *sqlx.DB
	ollamaService *ollama.Service
}

func ConnectDB(ollamaService *ollama.Service, isDev bool) (*Database, error) {
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
		Db:            db,
		ollamaService: ollamaService,
	}
	return &database, nil
}

func (database *Database) InsertMessage(message *Message) error {
	if message.ConversationId == -1 {
		title, err := database.ollamaService.GenerateConversationTitle(message.Content)
		if err != nil {
			return err
		}

		newConversation := Conversation{
			Title: title,
		}
		if err = database.ensureConversation(&newConversation); err != nil {
			return err
		}
		message.ConversationId = newConversation.ID
	}

	_, err := database.Db.NamedExec("INSERT INTO messages (role, content, conversation_id, datetime) VALUES (:role, :content, :conversation_id, :datetime)", message)
	return err
}

func (database *Database) SelectMessagesOfConvo(conversationId int) ([]Message, error) {
	var messages []Message
	err := database.Db.Select(&messages, "SELECT role, content FROM messages WHERE conversation_id = $1 ORDER BY datetime ASC", conversationId)
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (database *Database) SelectConversations() ([]Conversation, error) {
	var conversations []Conversation
	err := database.Db.Select(&conversations, "SELECT title FROM conversations")
	return conversations, err
}

func (database *Database) ensureConversation(conversation *Conversation) error {
	_, err := database.Db.NamedExec("INSERT OR IGNORE INTO conversations (title) VALUES (:title)", conversation)
	if err != nil {
		return err
	}

	err = database.Db.Get(conversation, "SELECT id, title FROM conversations WHERE title = $1", conversation.Title)
	return err
}
