package conversation

import (
	"akernel/internal/repository"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Repository provides methods to interact with persistence layer storage for conversation data.
type Repository struct {
	db *sqlx.DB
}

// New constructs and returns a new Repository implementation for data access operations.
func New(db *sqlx.DB) Repository {
	return Repository{
		db,
	}
}

// SelectConversations retrieves a list of all available conversations.
func (d *Repository) SelectConversations() ([]repository.Conversation, error) {
	var conversations []repository.Conversation
	err := d.db.Select(&conversations, "SELECT id, title FROM conversations")
	if err != nil {
		return nil, fmt.Errorf("select conversations: %w", err)
	}

	return conversations, nil
}

// EnsureConversation ensures a conversation exists in the database and populates its details.
func (d *Repository) EnsureConversation(conversation *repository.Conversation) error {
	_, err := d.db.NamedExec("INSERT OR IGNORE INTO conversations (title) VALUES (:title)", conversation)
	if err != nil {
		return fmt.Errorf("ensure conversation: %w", err)
	}

	err = d.db.Get(conversation, "SELECT id, title FROM conversations WHERE title = $1", conversation.Title)
	if err != nil {
		return fmt.Errorf("ensure conversation: %w", err)
	}

	return nil
}

// DoesConversationExist verifies if a conversation exists in the database.
func (d *Repository) DoesConversationExist(conversationId int) bool {
	var convo repository.Conversation
	err := d.db.Get(&convo, "SELECT id FROM conversations WHERE id = $1", conversationId)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}

	return convo.ID == conversationId
}

// CascadeDeleteConversation does a cascade delete on a conversation in the database.
func (d *Repository) CascadeDeleteConversation(conversationId int) (int, error) {
	res, err := d.db.Exec("DELETE FROM conversations WHERE id = $1", conversationId)
	if err != nil {
		return 0, fmt.Errorf("delete conversation: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete conversation: %w", err)
	}

	return int(rowsAffected), nil
}

// SetNewSummary updates the last_summary field of a conversation with the specified summaryId in the database.
func (d *Repository) SetNewSummary(conversationId, summaryId int) error {
	_, err := d.db.Exec("UPDATE conversations SET last_summary = $1 WHERE id = $2", summaryId, conversationId)
	if err != nil {
		return fmt.Errorf("set new summary: %w", err)
	}

	return nil
}
