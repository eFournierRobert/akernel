// Package inference provides the top-level abstraction for LLM inference.
// It sits between the service layer and provider-specific clients, handling
// model resolution, system prompt injection, and provider dispatch routing.
package inference

import (
	"akernel/internal/config"
	"akernel/internal/service"
	"akernel/internal/service/inference/ollama"
	"akernel/internal/service/inference/openrouter"
	"akernel/internal/service/inference/tools"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
)

// Service holds the system prompt, configured models, and provider-specific clients.
// It acts as the facade between the service layer and lower-level inference providers.
type Service struct {
	systemPrompt      HistoryMessage
	models            []config.Model
	ollamaService     ollamaClient
	openrouterService openrouterClient
}

// New creates a new Service from the given configuration, initializing the system prompt and model list.
func New(config config.Config) Service {
	systemPrompt := HistoryMessage{
		Role:    "system",
		Content: config.SystemPrompt,
	}

	return Service{
		systemPrompt:      systemPrompt,
		models:            config.Models,
		ollamaService:     ollama.New(),
		openrouterService: openrouter.New(config),
	}
}

// GetModels returns the list of configured AI models.
func (s Service) GetModels() []config.Model {
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
func (s Service) PostChat(ctx context.Context, model string, history []HistoryMessage, thinking bool) (MessageResponse, error) {
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
	case "openrouter":
		response, err := s.openrouterService.PostChat(ctx, fullModel.Id, s.convertFullHistoryToOpenrouterFormat(fullHistory))
		if err != nil {
			return MessageResponse{}, err
		}

		return MessageResponse{
			Model:            model,
			Response:         s.convertOpenrouterHistoryToInferenceFormat(response.Choices[0].Message),
			TokenCount:       response.Usage.TotalTokens,
			PromptTokenCount: response.Usage.PromptTokens,
		}, nil
	}

	return MessageResponse{}, errors.New("invalid provider")
}

// GenerateConversationTitle produces a short title for a user message by sending a summary prompt to the model.
// It returns an error if the inference call fails or returns empty content.
func (s Service) GenerateConversationTitle(ctx context.Context, message, model string) (string, error) {
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

func (s Service) convertOllamaHistoryToInferenceFormat(h ollama.HistoryMessage) HistoryMessage {
	return HistoryMessage{
		Role:      h.Role,
		Content:   h.Content,
		Thinking:  h.Thinking,
		ToolCalls: h.ToolCalls,
	}
}

func (s Service) convertOpenrouterHistoryToInferenceFormat(h openrouter.HistoryMessage) HistoryMessage {
	if len(h.ToolCalls) > 0 {
		for i, t := range h.ToolCalls {
			var args string
			if err := json.Unmarshal(t.Function.Arguments, &args); err != nil {
				h.ToolCalls = nil
				break
			}

			h.ToolCalls[i].Function.Arguments = json.RawMessage(args)
		}
	}

	if h.Content == nil {
		emptyString := ""
		h.Content = &emptyString
	}

	return HistoryMessage{
		Role:      h.Role,
		Content:   *h.Content,
		ToolCalls: h.ToolCalls,
	}
}

func (s Service) convertFullHistoryToOllamaFormat(history []HistoryMessage) []ollama.HistoryMessage {
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

// convertFullHistoryToOpenrouterFormat transforms internal message history into the format expected by OpenRouter's API.
// It handles extracting tool calls from assistant responses, converting JSON arguments to properly escaped strings,
// and managing optional content fields (setting Content to nil pointer when empty). The function also sets Type="function"
// on all ToolCalls for proper OpenRouter formatting.
func (s Service) convertFullHistoryToOpenrouterFormat(history []HistoryMessage) []openrouter.HistoryMessage {
	var h []openrouter.HistoryMessage
	for _, m := range history {
		if m.Role == "assistant" {
			var calls []tools.ToolCall
			if err := json.Unmarshal([]byte(m.Content), &calls); err == nil {
				m.Content = ""

				for i, c := range calls {
					stringifyJson, err := json.Marshal(string(c.Function.Arguments))
					if err != nil {
						log.Printf("Failed to convert tool call for openrouter format: %s\n", err.Error())
						continue
					}
					calls[i].Function.Arguments = stringifyJson
					calls[i].Type = "function"
				}
				m.ToolCalls = append(m.ToolCalls, calls...)
			}
		}

		if m.Content == "" {
			h = append(h, openrouter.HistoryMessage{
				Role:      m.Role,
				Content:   nil,
				ToolCalls: m.ToolCalls,
			})
			continue
		}

		h = append(h, openrouter.HistoryMessage{
			Role:       m.Role,
			Content:    &m.Content,
			ToolCallId: m.ToolCallId,
			ToolCalls:  m.ToolCalls,
		})
	}

	return h
}
