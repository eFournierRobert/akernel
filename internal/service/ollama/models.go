// Package ollama defines the types used by the ollama client.
package ollama

import (
	"akernel/internal/service/ollama/tools"
)

// Model represents metadata for an AI model available in Ollama.
type Model struct {
	Name        string
	Description string
	RealName    string
}

// HistoryMessage represents a single message within a conversation history, including its role and content.
type HistoryMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Thinking  string           `json:"thinking"`
	ToolCalls []tools.ToolCall `json:"tool_calls"`
}

// Message defines the request payload for sending a chat prompt to an Ollama model.
type Message struct {
	Model     string           `json:"model"`
	Messages  []HistoryMessage `json:"messages"`
	Think     bool             `json:"think"`
	KeepAlive string           `json:"keep_alive"`
	Stream    bool             `json:"stream"`
	Options   Options          `json:"options"`
	Tools     []tools.Tools    `json:"tools"`
}

// Options provides configuration parameters for the AI model's generation process.
type Options struct {
	Temperature float32 `json:"temperature"`
	NumCtx      int     `json:"num_ctx"`
}

// MessageResponse represents a single chunk of a streaming response from the Ollama API.
type MessageResponse struct {
	Model            string         `json:"model"`
	Done             bool           `json:"done"`
	Response         HistoryMessage `json:"message"`
	TokenCount       int            `json:"eval_count"`
	PromptTokenCount int            `json:"prompt_eval_count"`
}
