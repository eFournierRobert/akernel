package chat

import (
	"akernel/internal/database"
	"akernel/internal/handlers/dto"
	"akernel/internal/service/ollama"
	"encoding/json"
	"errors"
	"time"
)

// Service handles the business logic for chat operations, acting as an intermediary between the handlers and the database.
type Service struct {
	database      *database.Database
	ollamaService *ollama.Service
}

// NewChatService initializes a new instance of the Chat Service.
func NewChatService(database *database.Database, ollama *ollama.Service) *Service {
	service := Service{
		database:      database,
		ollamaService: ollama,
	}

	return &service
}

func (s *Service) GeneratePromptAnswer(prompt *dto.Prompt) (*dto.Answer, error) {
	conversationId, err := s.insertPrompt(prompt)
	if err != nil {
		return nil, err
	}

	history, err := s.fetchConversationHistory(conversationId)
	if err != nil {
		return nil, err
	}

	rawAnswer, err := s.ollamaService.PostChat(prompt.Message, prompt.Model, history)
	if err != nil {
		return nil, err
	}

	var answer ollama.MessageFormat
	if err = json.Unmarshal(rawAnswer, &answer); err != nil {
		return nil, err
	}

	if len(answer.MessageBody) > 0 && answer.MessageBody[0].Body != "" {
		if err = s.InsertAssistantAnswer(rawAnswer, conversationId); err != nil {
			return nil, err
		}
		return &dto.Answer{
			Message:        answer,
			ConversationId: conversationId,
		}, nil
	}

	if err = s.database.PopLastUserMessage(conversationId); err != nil {
		return nil, err
	}
	return nil, errors.New("received empty answer from ollama")
}

// InsertPrompt saves a user's message to the database and returns the associated conversation ID.
func (s *Service) insertPrompt(prompt *dto.Prompt) (int, error) {
	message := database.Message{
		Role:           "user",
		Content:        prompt.Message,
		ConversationId: prompt.ConversationId,
		DateTime:       time.Now(),
	}

	if message.ConversationId == -1 {
		title, err := s.ollamaService.GenerateConversationTitle(message.Content)
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

// FetchConversationHistory retrieves all messages for a conversation and converts them into the history format required by Ollama.
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

// FetchConversations retrieves a list of all conversations available in the database and returns them as DTOs.
func (s *Service) FetchConversations() ([]dto.Conversation, error) {
	conversations, err := s.database.SelectConversations()
	if err != nil {
		return nil, err
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
