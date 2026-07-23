// Package inference provides the top-level abstraction for LLM inference.
// It sits between the service layer and provider-specific clients, handling
// model resolution, system prompt injection, and provider dispatch routing.
package inference

import (
	"akernel/internal/config"
	"akernel/internal/service"
	"akernel/internal/service/inference/ollama"
	"context"
	"errors"
	"fmt"
)

// Service holds the system prompt, configured models, and provider-specific clients.
// It acts as the facade between the service layer and lower-level inference providers.
type Service struct {
	systemPrompt  HistoryMessage
	models        []config.Model
	ollamaService ollama.Service
}

// New creates a new Service from the given configuration, initializing the system prompt and model list.
func New(config config.Config) Service {
	systemPrompt := HistoryMessage{
		Role:    "system",
		Content: config.SystemPrompt,
	}

	return Service{
		systemPrompt:  systemPrompt,
		models:        config.Models,
		ollamaService: ollama.New(),
	}
}

// GetModels returns the list of configured AI models.
func (s *Service) GetModels() []config.Model {
	return s.models
}

// GetModel returns the model configuration matching the given name.
// It returns ErrModelNotFound if no configured model matches.
func (s *Service) GetModel(modelName string) (config.Model, error) {
	for _, m := range s.models {
		if m.Name == modelName {
			return m, nil
		}
	}

	return config.Model{}, service.ErrModelNotFound
}

// PostChat resolves the requested model, prepends the system prompt to the conversation history,
// and dispatches the request to the appropriate provider. It returns the completed response or an error.
func (s *Service) PostChat(ctx context.Context, model string, history []HistoryMessage, thinking bool) (MessageResponse, error) {
	fullModel, err := s.GetModel(model)
	if err != nil {
		return MessageResponse{}, err
	}

	fullHistory := make([]HistoryMessage, 1, len(history)+1)
	fullHistory[0] = s.systemPrompt
	fullHistory = append(fullHistory, history...)

	switch fullModel.Provider {
	case "ollama":
		response, err := s.ollamaService.PostChat(ctx, fullModel.Id, s.convertFullHistoryToOllamaFormat(fullHistory), thinking)
		if err != nil {
			return MessageResponse{}, err
		}

		return MessageResponse{
			Model:            model,
			Response:         s.convertOllamaHistoryToInferenceFormat(response.Response),
			TokenCount:       response.TokenCount,
			PromptTokenCount: response.PromptTokenCount,
		}, nil
	}

	return MessageResponse{}, errors.New("invalid provider")
}

// GenerateConversationTitle produces a short title for a user message by sending a summary prompt to the model.
// It returns an error if the inference call fails or returns empty content.
func (s *Service) GenerateConversationTitle(ctx context.Context, message, model string) (string, error) {
	titlePrompt := fmt.Sprintf("Generate a small conversation title for this message. Do not make propositions. Just answer with the title. Message: %s", message)

	history := []HistoryMessage{{Role: "user", Content: titlePrompt}}

	answer, err := s.PostChat(ctx, model, history, false)
	if err != nil {
		return "", fmt.Errorf("error while generating title: %w", err)
	}

	if len(answer.Response.Content) == 0 {
		return "", fmt.Errorf("error while generating title: %w", errors.New("empty response when requested for a title"))
	}
	return answer.Response.Content, nil
}

func (s *Service) convertOllamaHistoryToInferenceFormat(h ollama.HistoryMessage) HistoryMessage {
	return HistoryMessage{
		Role:      h.Role,
		Content:   h.Content,
		Thinking:  h.Thinking,
		ToolCalls: h.ToolCalls,
	}
}

func (s *Service) convertFullHistoryToOllamaFormat(history []HistoryMessage) []ollama.HistoryMessage {
	var h []ollama.HistoryMessage
	for _, m := range history {
		h = append(h, ollama.HistoryMessage{
			Role:      m.Role,
			Content:   m.Content,
			ToolCalls: m.ToolCalls,
		})
	}

	return h
}
