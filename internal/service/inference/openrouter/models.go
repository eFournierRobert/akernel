// Package openrouter provides OpenRouter API request and response types for chat completions
// with function calling tool support. It defines the data structures used to communicate
// with OpenRouter's v1 API endpoints.
package openrouter

import "akernel/internal/service/inference/tools"

// HistoryMessage represents a single message in the conversation history for OpenRouter API compatibility.
// The Role field identifies whether this is a "user", "assistant", or "system" message.
// Content may be nil when the message only contains tool call references (e.g., assistant responses with function calls).
type HistoryMessage struct {
	Role       string           `json:"role"`
	Content    *string          `json:"content"`
	ToolCallId string           `json:"tool_call_id"`
	ToolCalls  []tools.ToolCall `json:"tool_calls"`
}

// Message defines the request payload structure for OpenRouter's /chat/completions endpoint.
// It includes the model ID, message history, streaming preference, and available tool schemas
// that the AI can call during generation.
type Message struct {
	Model    string           `json:"model"`
	Messages []HistoryMessage `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []tools.Tools    `json:"tools"`
}

// MessageResponse represents the parsed response from OpenRouter's chat completions API.
// It contains one or more completion choices, each with a message and token usage statistics.
type MessageResponse struct {
	Choices []struct {
		Message HistoryMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}
