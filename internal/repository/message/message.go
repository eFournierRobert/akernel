// Package message provides database access methods for chat messages.
package message

import (
	"akernel/internal/repository"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Repository provides database access methods for chat conversation data.
type Repository struct {
	db *sqlx.DB
}

// New creates a new Repository instance using the provided database connection.
func New(db *sqlx.DB) Repository {
	return Repository{
		db,
	}
}

// InsertMessage saves a message to the database and creates a new conversation if necessary.
func (d *Repository) InsertMessage(message *repository.Message) error {
	_, err := d.db.NamedExec("INSERT INTO messages (role, content, conversation_id, datetime, token_count) VALUES (:role, :content, :conversation_id, :datetime, :token_count)", message)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	return nil
}

// SelectMessagesOfConvo retrieves all messages for a specific conversation, ordered by date.
func (d *Repository) SelectMessagesOfConvo(conversationId int) ([]repository.Message, error) {
	var conversation repository.Conversation
	err := d.db.Get(&conversation, "SELECT last_summary FROM conversations WHERE id = $1", conversationId)
	if err != nil {
		return nil, fmt.Errorf("select messages of conversation: %w", err)
	}

	var messages []repository.Message
	if conversation.LastSummary.Valid {
		var summary repository.Message
		err = d.db.Get(&summary, "SELECT id, role, content FROM messages WHERE id = $1", conversation.LastSummary)
		if err != nil {
			return nil, fmt.Errorf("select messages of conversation: %w", err)
		}

		err = d.db.Select(&messages, "SELECT role, content FROM messages WHERE conversation_id = $1 AND id > $2 ORDER BY id", conversationId, summary.ID)
		if err != nil {
			return nil, fmt.Errorf("select messages of conversation: %w", err)
		}

		messages = append([]repository.Message{summary}, messages...)
	} else {
		err = d.db.Select(&messages, "SELECT role, content FROM messages WHERE conversation_id = $1 ORDER BY id", conversationId)
		if err != nil {
			return nil, fmt.Errorf("select messages of conversation: %w", err)
		}
	}

	return messages, nil
}

// PopLastUserMessage removes the most recent message sent by the user in a specific conversation.
func (d *Repository) PopLastUserMessage(conversationId int) error {
	_, err := d.db.Exec(
		"DELETE FROM messages WHERE id = (SELECT id FROM messages WHERE role = 'user' AND conversation_id = $1 ORDER BY datetime DESC LIMIT 1)",
		conversationId)
	if err != nil {
		return fmt.Errorf("pop last user message: %w", err)
	}

	return nil
}

// SelectFullConversation fetches all the messages of a conversation by its ID.
func (d *Repository) SelectFullConversation(conversationId int) ([]repository.Message, error) {
	var messages []repository.Message
	err := d.db.Select(&messages, "SELECT role, content FROM messages WHERE conversation_id = $1 ORDER BY datetime", conversationId)
	if err != nil {
		return nil, fmt.Errorf("get full conversation: %w", err)
	}

	return messages, nil
}

// AddTokenToLatestUserMessage updates the token count for the most recent user message in a conversation.
func (d *Repository) AddTokenToLatestUserMessage(conversationId, tokenCount int) error {
	_, err := d.db.Exec(
		"UPDATE messages SET token_count = $1 WHERE id = (SELECT id FROM messages WHERE conversation_id = $2 AND role='user' ORDER BY datetime DESC LIMIT 1)",
		tokenCount,
		conversationId)
	if err != nil {
		return fmt.Errorf("updating latest user message token count: %w", err)
	}

	return nil
}

// SelectLastAssistantMessage retrieves the most recent message posted by an assistant for a given conversation ID.
func (d *Repository) SelectLastAssistantMessage(conversationId int) (repository.Message, error) {
	var message repository.Message
	err := d.db.Get(&message, "SELECT id FROM messages WHERE conversation_id = $1 AND role = 'assistant' ORDER BY id DESC LIMIT 1", conversationId)
	if err != nil {
		return repository.Message{}, fmt.Errorf("select last assistant message: %w", err)
	}

	return message, nil
}

// SelectAllMessagesAfterLastPrompt retrieves all messages from a conversation that were created
// on or after the most recent user message matching the given prompt text. This includes tool
// call responses and subsequent assistant messages following the prompt.
func (d *Repository) SelectAllMessagesAfterLastPrompt(conversationId int, prompt string) ([]repository.Message, error) {
	var messages []repository.Message
	err := d.db.Select(&messages, "SELECT role, content, token_count, conversation_id FROM messages WHERE id >= (SELECT MIN(id) FROM messages WHERE content = $1 AND role = 'user' AND conversation_id = $2) AND conversation_id = $2 ORDER BY id", prompt, conversationId)
	if err != nil {
		return nil, fmt.Errorf("select all messages after last prompt: %w", err)
	}
	return messages, nil
}
