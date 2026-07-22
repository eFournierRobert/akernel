package service

import (
	"akernel/internal/service/ollama"
	"context"
)

// OllamaClient abstracts the Ollama service for model retrieval and chat, allowing mocking in tests.
type OllamaClient interface {
	GetModels() []ollama.Model
	PostChat(ctx context.Context, modelName string, history []ollama.HistoryMessage, thinking bool) (ollama.MessageResponse, error)
	GenerateConversationTitle(ctx context.Context, message, model string) (string, error)
	GetModelId(modelName string) string
}
