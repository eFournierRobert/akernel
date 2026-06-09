package service

import (
	"akernel/internal/database"
	"akernel/internal/handlers/dto"
	"akernel/internal/ollama"
	"time"
)

type ChatService struct {
	database *database.Database
}

func NewChatService(database *database.Database) *ChatService {
	service := ChatService{
		database: database,
	}

	return &service
}

func (service *ChatService) InsertPrompt(prompt *dto.Prompt) (int, error) {
	message := database.Message{
		Role:           "user",
		Content:        prompt.Message,
		ConversationId: prompt.ConversationId,
		DateTime:       time.Now(),
	}

	if err := service.database.InsertMessage(&message); err != nil {
		return 0, err
	}

	return message.ConversationId, nil
}

func (service *ChatService) InsertAssistantAnswer(answer []byte, conversationId int) error {
	message := database.Message{
		Role:           "assistant",
		Content:        string(answer),
		ConversationId: conversationId,
		DateTime:       time.Now(),
	}

	return service.database.InsertMessage(&message)
}

func (service *ChatService) FetchConversationHistory(conversationId int) ([]ollama.HistoryMessage, error) {
	messages, err := service.database.SelectMessagesOfConvo(conversationId)
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

func (service *ChatService) FetchConversations() ([]dto.Conversation, error) {
	conversations, err := service.database.SelectConversations()
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

func (service *ChatService) PopLastUserMessage(conversationId int) error {
	return service.database.PopLastUserMessage(conversationId)
}
