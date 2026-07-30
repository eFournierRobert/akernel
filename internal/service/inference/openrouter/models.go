package openrouter

import "akernel/internal/service/inference/tools"

type HistoryMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []tools.ToolCall `json:"tool_calls"`
}

type Message struct {
	Model    string           `json:"model"`
	Messages []HistoryMessage `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []tools.Tools    `json:"tools"`
}

type MessageResponse struct {
	Choices []struct {
		Message HistoryMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}
