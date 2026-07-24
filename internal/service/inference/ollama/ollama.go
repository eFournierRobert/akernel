// Package ollama provides a client for interacting with the Ollama API, including chat requests and model listing.
package ollama

import (
	"akernel/internal/service/inference/tools"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ollamaBasePath is the base URL for the Ollama API.
var ollamaBasePath = "http://localhost:11434/api"

// Service provides a client for interacting with the Ollama AI service.
type Service struct {
}

// New creates a new Ollama service instance.
func New() Service {
	ollamaService := Service{}

	return ollamaService
}

// PostChat sends a message along with conversation history to the specified Ollama model and returns the generated response.
func (s Service) PostChat(ctx context.Context, modelName string, history []HistoryMessage, thinking bool) (MessageResponse, error) {
	body := Message{
		Model:     modelName,
		Messages:  history,
		Think:     thinking,
		KeepAlive: "5m",
		Stream:    false,
		Options: Options{
			Temperature: 0.8,
			NumCtx:      32000,
		},
		Tools: tools.GetAllToolSchemas(),
	}

	jsonBody, err := json.Marshal(&body)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("post chat: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildPath("/chat"), bytes.NewBuffer(jsonBody))
	if err != nil {
		return MessageResponse{}, fmt.Errorf("post chat: %w", err)
	}

	client := http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("post chat: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		var fullAnswer MessageResponse
		answer, err := io.ReadAll(response.Body)
		if err != nil {
			return MessageResponse{}, fmt.Errorf("post chat: %w", err)
		}

		if err := json.Unmarshal(answer, &fullAnswer); err != nil {
			return MessageResponse{}, fmt.Errorf("post chat: %w", err)
		}

		return fullAnswer, nil
	}
	b, err := io.ReadAll(response.Body)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("post chat: %w", err)
	}
	return MessageResponse{}, fmt.Errorf("post chat: %w", errors.New(string(b)))
}

// buildPath constructs a full URL for the Ollama API endpoints.
func buildPath(path string) string {
	return fmt.Sprintf("%s%s", ollamaBasePath, path)
}
