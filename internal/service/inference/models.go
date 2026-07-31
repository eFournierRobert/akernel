package inference

import (
	"akernel/internal/config"
	"akernel/internal/service/inference/ollama"
	"akernel/internal/service/inference/openrouter"
	"akernel/internal/service/inference/tools"
	"context"
)

// InferenceClient abstracts an inference provider's chat capability, allowing mocking in tests.
type InferenceClient interface {
	PostChat(ctx context.Context, modelName string, history []HistoryMessage, thinking bool) (MessageResponse, error)
	GetModels() []config.Model
	GenerateConversationTitle(ctx context.Context, message, model string) (string, error)
}

// ollamaClient abstracts the Ollama provider so it can be mocked in tests.
type ollamaClient interface {
	PostChat(ctx context.Context, modelName string, history []ollama.HistoryMessage, thinking bool) (ollama.MessageResponse, error)
}

// openrouterClient abstracts the OpenRouter provider so it can be mocked in tests.
type openrouterClient interface {
	PostChat(ctx context.Context, modelName string, history []openrouter.HistoryMessage) (openrouter.MessageResponse, error)
}

// HistoryMessage represents a single message within a conversation history, including its role and content.
type HistoryMessage struct {
	Role       string
	Content    string
	Thinking   string
	ToolCallId string
	ToolCalls  []tools.ToolCall
}

// Message defines the request payload for sending a chat prompt to an LLM provider.
type Message struct {
	ModelId  string
	Messages []HistoryMessage
	Think    bool
	Tools    []tools.Tools
}

// MessageResponse represents the completed response returned by an LLM provider after a chat request.
type MessageResponse struct {
	Model            string
	Response         HistoryMessage
	TokenCount       int
	PromptTokenCount int
}
