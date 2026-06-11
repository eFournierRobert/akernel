package chat

import (
	"akernel/internal/database"
	"akernel/internal/handlers/dto"
	"akernel/internal/service"
	"akernel/internal/service/ollama"
	"context"
	"errors"
	"fmt"
	"time"
)

// Service handles the business logic for chat operations, acting as an intermediary between the handlers and the database.
type Service struct {
	database      *database.Database
	ollamaService *ollama.Service
}

// NewChatService initializes a new instance of the Chat Service.
func NewChatService(database *database.Database, ollama *ollama.Service) *Service {
	s := Service{
		database:      database,
		ollamaService: ollama,
	}

	return &s
}

// GeneratePromptAnswer takes a user prompt and generates an AI answer. Also creates a conversation if necessary.
func (s *Service) GeneratePromptAnswer(ctx context.Context, prompt *dto.Prompt) (*dto.Answer, error) {
	if modelIsValid, err := s.doesModelExist(ctx, prompt.Model); err != nil {
		return nil, err
	} else if !modelIsValid {
		return nil, fmt.Errorf("asked for model %s: %w", prompt.Model, service.ErrModelNotFound)
	}

	if prompt.ConversationId < -1 || (prompt.ConversationId >= 1 && !s.database.DoesConversationExist(prompt.ConversationId)) {
		return nil, fmt.Errorf("generate prompt answer: %w", service.ErrInvalidConversation)
	}

	conversationId, err := s.insertPrompt(ctx, prompt)
	if err != nil {
		return nil, err
	}

	history, err := s.fetchConversationHistory(conversationId)
	if err != nil {
		return nil, err
	}

	answer, err := s.ollamaService.PostChat(ctx, prompt.Model, history, true)
	if err != nil || ctx.Err() != nil {
		if err = s.database.PopLastUserMessage(conversationId); err != nil {
			return nil, err
		}

		return nil, err
	}

	if len(answer) > 0 {
		if err = s.InsertAssistantAnswer(answer, conversationId); err != nil {
			return nil, err
		}
		return &dto.Answer{
			Message:        string(answer),
			ConversationId: conversationId,
		}, nil
	}

	if err = s.database.PopLastUserMessage(conversationId); err != nil {
		return nil, err
	}
	return nil, errors.New("received empty answer from ollama")
}

// InsertAssistantAnswer saves the AI's response to the database for a specific conversation.
func (s *Service) InsertAssistantAnswer(answer []byte, conversationId int) error {
	message := database.Message{
		Role:           "assistant",
		Content:        string(answer),
		ConversationId: conversationId,
		DateTime:       time.Now(),
	}

	return s.database.InsertMessage(&message)
}

// FetchConversations retrieves a list of all conversations available in the database and returns them as DTOs.
func (s *Service) FetchConversations() ([]dto.Conversation, error) {
	conversations, err := s.database.SelectConversations()
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

// GetModels returns a list of available AI models.
func (s *Service) GetModels(ctx context.Context) ([]dto.Model, error) {
	modelList, err := s.ollamaService.GetModels(ctx)
	if err != nil {
		return nil, err
	}

	var dtoModelList []dto.Model
	for _, model := range modelList.ModelList {
		dtoModelList = append(dtoModelList, dto.Model{
			Name:  model.Name,
			Model: model.Model,
			Size:  convertSizeToGB(model.Size),
		})
	}

	return dtoModelList, nil
}

// DeleteConversation deletes a conversation and the related messages.
func (s *Service) DeleteConversation(conversationId int) error {
	rowsAffected, err := s.database.CascadeDeleteConversation(conversationId)
	if err != nil {
		return err
	} else if rowsAffected == 0 {
		return fmt.Errorf("generate prompt answer: %w", service.ErrConversationDoesNotExist)
	}

	return nil
}

// doesModelExist verifies that a model is downloaded.
func (s *Service) doesModelExist(ctx context.Context, modelName string) (bool, error) {
	modelList, err := s.ollamaService.GetModels(ctx)
	if err != nil {
		return false, err
	}

	for _, model := range modelList.ModelList {
		if model.Name == modelName {
			return true, nil
		}
	}

	return false, nil
}

// insertPrompt saves a user's message to the database and returns the associated conversation ID.
func (s *Service) insertPrompt(ctx context.Context, prompt *dto.Prompt) (int, error) {
	message := database.Message{
		Role:           "user",
		Content:        prompt.Message,
		ConversationId: prompt.ConversationId,
		DateTime:       time.Now(),
	}

	if message.ConversationId == -1 {
		title, err := s.ollamaService.GenerateConversationTitle(ctx, message.Content, prompt.Model)
		if err != nil {
			return 0, err
		}

		newConversation := database.Conversation{
			Title: title,
		}
		if err = s.database.EnsureConversation(&newConversation); err != nil {
			return 0, err
		}
		message.ConversationId = newConversation.ID
	}

	if err := s.database.InsertMessage(&message); err != nil {
		return 0, err
	}

	return message.ConversationId, nil
}

// fetchConversationHistory retrieves all messages for a conversation and converts them into the history format required by Ollama.
func (s *Service) fetchConversationHistory(conversationId int) ([]ollama.HistoryMessage, error) {
	messages, err := s.database.SelectMessagesOfConvo(conversationId)
	if err != nil {
		return nil, err
	}

	var history []ollama.HistoryMessage
	for _, message := range messages {
		history = append(history, ollama.HistoryMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}

	return history, nil
}

// convertSizeToGB converts a model size from bytes to gigabytes.
func convertSizeToGB(bytes int) float32 {
	return float32(bytes) / float32(1073741824)
}
