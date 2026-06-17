package conversation

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/repository"
	"akernel/internal/repository/conversation"
	"akernel/internal/repository/message"
	"akernel/internal/service"
	"akernel/internal/service/ollama"
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Service struct {
	repository  conversation.Repository
	messageRepo message.Repository
	ollama      ollama.Service
}

func New(db *sqlx.DB, ollamaService ollama.Service) Service {
	return Service{
		repository:  conversation.New(db),
		messageRepo: message.New(db),
		ollama:      ollamaService,
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

func (s *Service) CreateNewSummary(ctx context.Context, conversationId int, modelName string) error {
	if !s.repository.DoesConversationExist(conversationId) {
		return fmt.Errorf("create new summary: %w", service.ErrConversationDoesNotExist)
	}

	fullConvo, err := s.selectFullConvoIntoOllamaHistoryMessages(conversationId)
	if err != nil {
		return err
	}

	fullConvo = append(fullConvo, ollama.HistoryMessage{
		Role:    "user",
		Content: "Summarize the whole conversation into one concise message.",
	})

	res, err := s.ollama.PostChat(ctx, modelName, fullConvo, true)
	if err != nil {
		return err
	}
	if len(res.Response.Content) == 0 {
		return fmt.Errorf("create new summary: %w", errors.New("AI answered with an empty message"))
	}

	newSummary := repository.Message{
		Role:           res.Response.Role,
		Content:        res.Response.Content,
		TokenCount:     res.TokenCount,
		ConversationId: conversationId,
	}
	if err = s.messageRepo.InsertMessage(&newSummary); err != nil {
		return err
	}

	newSummary, err = s.messageRepo.SelectLastAssistantMessage(conversationId)
	if err != nil {
		return err
	}

	if err = s.repository.SetNewSummary(conversationId, newSummary.ID); err != nil {
		return err
	}

	return nil
}

func (s *Service) selectFullConvoIntoOllamaHistoryMessages(conversationId int) ([]ollama.HistoryMessage, error) {
	messages, err := s.messageRepo.SelectFullConversation(conversationId)
	if err != nil {
		return nil, err
	}

	var fullConvo []ollama.HistoryMessage
	for _, m := range messages {
		fullConvo = append(fullConvo, ollama.HistoryMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return fullConvo, nil
}
