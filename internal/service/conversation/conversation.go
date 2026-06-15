package conversation

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/repository/conversation"
	"akernel/internal/service"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Service struct {
	repository conversation.Repository
}

func New(db *sqlx.DB) Service {
	return Service{
		repository: conversation.New(db),
	}
}

// FetchConversations retrieves a list of all conversations available in the database and returns them as DTOs.
func (s *Service) FetchConversations() ([]dto.Conversation, error) {
	conversations, err := s.repository.SelectConversations()
	if err != nil {
		return nil, err
	}

	if conversations == nil {
		return []dto.Conversation{}, nil
	}

	var conversationDto []dto.Conversation
	for _, c := range conversations {
		conversationDto = append(conversationDto, dto.Conversation{
			Id:    c.ID,
			Title: c.Title,
		})
	}

	return conversationDto, nil
}

// DeleteConversation deletes a conversation and the related messages.
func (s *Service) DeleteConversation(conversationId int) error {
	rowsAffected, err := s.repository.CascadeDeleteConversation(conversationId)
	if err != nil {
		return err
	} else if rowsAffected == 0 {
		return fmt.Errorf("delete conversation: %w", service.ErrConversationDoesNotExist)
	}

	return nil
}
