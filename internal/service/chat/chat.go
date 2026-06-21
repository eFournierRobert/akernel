// Package chat implements the business logic for chat operations and tool-call orchestration.
package chat

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/repository"
	"akernel/internal/repository/conversation"
	"akernel/internal/repository/message"
	"akernel/internal/service"
	"akernel/internal/service/ollama"
	"akernel/internal/service/ollama/tools"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Service handles the business logic for chat operations, acting as an intermediary between the handlers and the database.
type Service struct {
	messageRepo   message.Repository
	convoRepo     conversation.Repository
	ollamaService service.OllamaClient
}

// NewChatService initializes a new instance of the Chat Service.
func NewChatService(db *sqlx.DB, ollama service.OllamaClient) *Service {
	s := Service{
		messageRepo:   message.New(db),
		convoRepo:     conversation.New(db),
		ollamaService: ollama,
	}

	return &s
}

// GeneratePromptAnswer takes a user prompt and generates an AI answer. Also creates a conversation if necessary.
func (s *Service) GeneratePromptAnswer(ctx context.Context, prompt *dto.Prompt) ([]dto.Answer, error) {
	if modelIsValid, err := s.doesModelExist(ctx, prompt.Model); err != nil {
		return nil, err
	} else if !modelIsValid {
		return nil, fmt.Errorf("asked for model %s: %w", prompt.Model, service.ErrModelNotFound)
	}

	if prompt.ConversationId < -1 || (prompt.ConversationId >= 0 && !s.convoRepo.DoesConversationExist(prompt.ConversationId)) {
		return nil, fmt.Errorf("generate prompt answer: %w", service.ErrInvalidConversation)
	}

	conversationId, err := s.insertPrompt(ctx, prompt)
	if err != nil {
		return nil, err
	}

	answer, err := s.getHistoryAndPostChat(ctx, conversationId, prompt)
	if err != nil || ctx.Err() != nil {
		if err := s.messageRepo.PopLastUserMessage(conversationId); err != nil {
			return nil, err
		}

		return nil, err
	}

	if len(answer.Response.ToolCalls) > 0 {
		if err = s.makeToolCalls(answer.Response.Thinking, conversationId, answer.Response.ToolCalls); err != nil {
			return nil, err
		}

		answer, err = s.getHistoryAndPostChat(ctx, conversationId, prompt)
		if err != nil || ctx.Err() != nil {
			if err := s.messageRepo.PopLastUserMessage(conversationId); err != nil {
				return nil, err
			}

			return nil, err
		}
	}

	if len(answer.Response.Content) > 0 {
		if err = s.InsertAssistantAnswer(answer.Response.Thinking, answer.Response.Content, answer.TokenCount, conversationId); err != nil {
			return nil, err
		}
		if err = s.messageRepo.AddTokenToLatestUserMessage(conversationId, answer.PromptTokenCount); err != nil {
			return nil, err
		}
		return s.getAllMessagesAfterPrompt(conversationId, prompt.Message)
	}

	if err = s.messageRepo.PopLastUserMessage(conversationId); err != nil {
		return nil, err
	}
	return nil, errors.New("received empty answer from ollama")
}

// InsertAssistantAnswer saves the AI's response to the database for a specific conversation.
func (s *Service) InsertAssistantAnswer(thinking, answer string, tokenCount, conversationId int) error {
	m := repository.Message{
		Role:           "assistant",
		Content:        answer,
		ConversationId: conversationId,
		Thinking:       thinking,
		DateTime:       time.Now(),
		TokenCount:     tokenCount,
	}

	return s.messageRepo.InsertMessage(m)
}

// GetConversation gets the conversation and returns an array of Message DTO.
func (s *Service) GetConversation(conversationId int) ([]dto.Message, error) {
	if !s.convoRepo.DoesConversationExist(conversationId) {
		return nil, fmt.Errorf("get full conversation: %w", service.ErrConversationDoesNotExist)
	}

	messages, err := s.messageRepo.SelectFullConversation(conversationId)
	if err != nil {
		return nil, err
	}

	var messagesDto []dto.Message
	if messages != nil {
		for _, m := range messages {
			messagesDto = append(messagesDto, dto.Message{
				Role:     m.Role,
				Thinking: m.Thinking,
				Content:  m.Content,
			})
		}
	} else {
		messagesDto = []dto.Message{}
	}

	return messagesDto, nil
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
	m := repository.Message{
		Role:           "user",
		Content:        prompt.Message,
		ConversationId: prompt.ConversationId,
		DateTime:       time.Now(),
	}

	if m.ConversationId == -1 {
		title, err := s.ollamaService.GenerateConversationTitle(ctx, m.Content, prompt.Model)
		if err != nil {
			return 0, err
		}

		newConversation := repository.Conversation{
			Title: title,
		}
		if err = s.convoRepo.EnsureConversation(&newConversation); err != nil {
			return 0, err
		}
		m.ConversationId = newConversation.ID
	}

	if err := s.messageRepo.InsertMessage(m); err != nil {
		return 0, err
	}

	return m.ConversationId, nil
}

// insertToolAnswer saves a single tool execution result as a message in the conversation.
func (s *Service) insertToolAnswer(conversationId int, toolAnswer string) error {
	m := repository.Message{
		Role:           "tool",
		Content:        toolAnswer,
		ConversationId: conversationId,
		DateTime:       time.Now(),
	}

	if err := s.messageRepo.InsertMessage(m); err != nil {
		return err
	}

	return nil
}

// fetchConversationHistory retrieves all messages for a conversation and converts them into the history format required by Ollama.
func (s *Service) fetchConversationHistory(conversationId int) ([]ollama.HistoryMessage, error) {
	messages, err := s.messageRepo.SelectMessagesOfConvo(conversationId)
	if err != nil {
		return nil, err
	}

	var history []ollama.HistoryMessage
	for _, m := range messages {
		history = append(history, ollama.HistoryMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return history, nil
}

// getHistoryAndPostChat retrieves the conversation history and sends it to Ollama for a response.
func (s *Service) getHistoryAndPostChat(ctx context.Context, conversationId int, prompt *dto.Prompt) (ollama.MessageResponse, error) {
	history, err := s.fetchConversationHistory(conversationId)
	if err != nil {
		return ollama.MessageResponse{}, err
	}

	answer, err := s.ollamaService.PostChat(ctx, prompt.Model, history, true)
	if err != nil || ctx.Err() != nil {
		if err := s.messageRepo.PopLastUserMessage(conversationId); err != nil {
			return ollama.MessageResponse{}, err
		}

		return ollama.MessageResponse{}, err
	}

	return answer, err
}

// makeToolCalls iterates over the tool calls returned by Ollama, dispatches each through ToolCallRouter, and stores the results as messages.
func (s *Service) makeToolCalls(thinking string, conversationId int, toolCalls []tools.ToolCall) error {
	toolCallJson, err := json.Marshal(toolCalls)
	if err != nil {
		return fmt.Errorf("generate prompt answer: %w", err)
	}

	if err = s.InsertAssistantAnswer(thinking, string(toolCallJson), 0, conversationId); err != nil {
		return fmt.Errorf("generate prompt answer: %w", err)
	}

	for _, t := range toolCalls {
		res := tools.ToolCallRouter(t)
		if err = s.insertToolAnswer(conversationId, res); err != nil {
			return fmt.Errorf("generate prompt answer: %w", err)
		}
	}

	return nil
}

// getAllMessagesAfterPrompt fetches all messages generated after the user's original prompt,
// excluding the prompt itself.
func (s *Service) getAllMessagesAfterPrompt(conversationId int, prompt string) ([]dto.Answer, error) {
	messages, err := s.messageRepo.SelectAllMessagesAfterLastPrompt(conversationId, prompt)
	if err != nil {
		return nil, err
	}

	var dtoMessages []dto.Answer
	for _, m := range messages {
		dtoMessages = append(dtoMessages, dto.Answer{
			Role:                   m.Role,
			ConversationId:         m.ConversationId,
			Message:                m.Content,
			Thinking:               m.Thinking,
			ConversationTokenCount: m.TokenCount,
		})
	}

	return dtoMessages, nil
}
