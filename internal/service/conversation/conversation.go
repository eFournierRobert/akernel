// Package conversation implements the business logic for managing conversations and summaries.
package conversation

import (
	"akernel/internal/config"
	"akernel/internal/handlers/dto"
	"akernel/internal/repository"
	"akernel/internal/repository/conversation"
	"akernel/internal/repository/message"
	"akernel/internal/service"
	"akernel/internal/service/inference"
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Service holds the necessary repositories and services for conversations.
type Service struct {
	repository  conversation.Repository
	messageRepo message.Repository
	inference   inference.Service
}

// New constructs a new Service implementation, initializing repositories and the inference service.
func New(db *sqlx.DB, config config.Config) Service {
	return Service{
		repository:  conversation.New(db),
		messageRepo: message.New(db),
		inference:   inference.New(config),
	}
}

// FetchConversations retrieves a list of all conversations available in the database and returns them as DTOs.
func (s *Service) FetchConversations() ([]dto.Conversation, error) {
	conversations, err := s.repository.SelectConversations()
	if err != nil {
		return nil, err
	}

	conversationDto := make([]dto.Conversation, 0)
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

// CreateNewSummary generates a comprehensive summary of the given conversation using the specified AI model.
func (s *Service) CreateNewSummary(ctx context.Context, conversationId int, modelName string) error {
	if !s.repository.DoesConversationExist(conversationId) {
		return fmt.Errorf("create new summary: %w", service.ErrConversationDoesNotExist)
	}

	fullConvo, err := s.selectFullConvoIntoInferenceHistoryMessages(conversationId)
	if err != nil {
		return err
	}

	fullConvo = append(fullConvo, inference.HistoryMessage{
		Role:    "user",
		Content: "Summarize the whole conversation into one concise message.",
	})

	res, err := s.inference.PostChat(ctx, modelName, fullConvo, true)
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
	if err = s.messageRepo.InsertMessage(newSummary); err != nil {
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

// selectFullConvoIntoInferenceHistoryMessages retrieves all messages for a conversation ID and maps them into inference.HistoryMessage format.
func (s *Service) selectFullConvoIntoInferenceHistoryMessages(conversationId int) ([]inference.HistoryMessage, error) {
	messages, err := s.messageRepo.SelectFullConversation(conversationId)
	if err != nil {
		return nil, err
	}

	var fullConvo []inference.HistoryMessage
	for _, m := range messages {
		fullConvo = append(fullConvo, inference.HistoryMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return fullConvo, nil
}
