package dto

import "akernel/internal/ollama"

type Prompt struct {
	Model          string `json:"model"`
	Message        string `json:"message"`
	ConversationId int    `json:"conversationId"`
}

type Answer struct {
	ConversationId int                  `json:"conversationId"`
	Message        ollama.MessageFormat `json:"message"`
}

type Conversation struct {
	Title string `json:"title"`
}

type Model struct {
	Name  string  `json:"name"`
	Model string  `json:"model"`
	Size  float32 `json:"size"`
}
