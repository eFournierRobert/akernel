package inference

import (
	"akernel/internal/service/inference/ollama"
	"akernel/internal/service/inference/tools"
	"context"
)

// OllamaClient abstracts the Ollama service for model retrieval and chat, allowing mocking in tests.
type InferenceClient interface {
	PostChat(ctx context.Context, modelName string, history []ollama.HistoryMessage, thinking bool) (ollama.MessageResponse, error)
}

// HistoryMessage represents a single message within a conversation history, including its role and content.
type HistoryMessage struct {
	Role      string
	Content   string
	Thinking  string
	ToolCalls []tools.ToolCall
}

// Message defines the request payload for sending a chat prompt to an Ollama model.
type Message struct {
	ModelId  string
	Messages []HistoryMessage
	Think    bool
	Tools    []tools.Tools
}

// MessageResponse represents a single chunk of a streaming response from the Ollama API.
type MessageResponse struct {
	Model            string
	Response         HistoryMessage
	TokenCount       int
	PromptTokenCount int
}
