package chat

import (
	"akernel/internal/database"
	"akernel/internal/handlers/dto"
	"akernel/internal/service/ollama"
	"time"
)

// Service handles the business logic for chat operations, acting as an intermediary between the handlers and the database.
type Service struct {
	database *database.Database
}

// NewChatService initializes a new instance of the Chat Service.
func NewChatService(database *database.Database) *Service {
	service := Service{
		database: database,
	}

	return &service
}

// InsertPrompt saves a user's message to the database and returns the associated conversation ID.
func (s *Service) InsertPrompt(prompt *dto.Prompt) (int, error) {
	message := database.Message{
		Role:           "user",
		Content:        prompt.Message,
		ConversationId: prompt.ConversationId,
		DateTime:       time.Now(),
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
func (s *Service) FetchConversationHistory(conversationId int) ([]ollama.HistoryMessage, error) {
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

// PopLastUserMessage removes the most recent user message from a specific conversation.
func (s *Service) PopLastUserMessage(conversationId int) error {
	return s.database.PopLastUserMessage(conversationId)
}
