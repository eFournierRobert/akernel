package openrouter

import (
	"akernel/internal/config"
	"akernel/internal/service/inference/tools"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var openrouterBasePath = "https://openrouter.ai/api/v1"

type Service struct {
	config config.Config
}

func New(c config.Config) Service {
	return Service{
		config: c,
	}
}

func (s Service) PostChat(ctx context.Context, modelName string, history []HistoryMessage) (MessageResponse, error) {
	body := Message{
		Model:    modelName,
		Stream:   false,
		Tools:    tools.GetAllToolSchemas(),
		Messages: history,
	}

	jsonBody, err := json.Marshal(&body)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("post chat: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildPath("/chat/completions"), bytes.NewBuffer(jsonBody))
	if err != nil {
		return MessageResponse{}, fmt.Errorf("post chat: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.OpenRouterApiKey)
	req.Header.Set("Content-Type", "application/json")

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

func buildPath(path string) string {
	return fmt.Sprintf("%s%s", openrouterBasePath, path)
}
