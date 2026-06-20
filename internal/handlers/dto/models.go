// Package dto defines data transfer objects used between HTTP handlers and service layers.
package dto

// Prompt is the DTO for a received prompt.
type Prompt struct {
	Model          string `json:"model"`
	Message        string `json:"message"`
	ConversationId int    `json:"conversationId"`
}

// Answer is the DTO for an AI-generated answer.
type Answer struct {
	Role                   string `json:"role"`
	ConversationId         int    `json:"conversationId"`
	Message                string `json:"message"`
	ConversationTokenCount int    `json:"conversationTokenCount"`
}

// Conversation is the DTO for a conversation.
type Conversation struct {
	Id    int    `json:"id"`
	Title string `json:"title"`
}

// Model is the DTO for AI model information.
type Model struct {
	Name  string  `json:"name"`
	Model string  `json:"model"`
	Size  float32 `json:"size"`
}

// Message is the DTO for a message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ModelOnly is the DTO for only sending the name of an AI model.
type ModelOnly struct {
	Name string `json:"name"`
}
