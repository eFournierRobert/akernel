package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/adrg/xdg"
)

// ollamaBasePath is the base URL for the Ollama API.
const ollamaBasePath = "http://localhost:11434/api"

// Service provides a client for interacting with the Ollama AI service.
type Service struct {
	systemPrompt string
}

// New initializes a new Service by loading the system prompt from the configuration file.
func New() (*Service, error) {
	systemPromptPath, err := xdg.ConfigFile("akernel/system.md")
	if err != nil {
		return nil, err
	}

	f, err := os.Open(systemPromptPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	systemPrompt, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	ollamaService := Service{
		systemPrompt: string(systemPrompt),
	}

	return &ollamaService, nil
}

// GetModels retrieves a list of available AI models from the Ollama service.
func (s *Service) GetModels(ctx context.Context) (*Models, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildPath("/tags"), nil)
	if err != nil {
		return nil, fmt.Errorf("get models from ollama: %w", err)
	}

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get models from ollama: %w", err)
	}
	defer res.Body.Close()

	var models Models
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("get models from ollama: %w", err)
	}

	err = json.Unmarshal(body, &models)
	if err != nil {
		return nil, fmt.Errorf("get models from ollama: %w", err)
	}

	return &models, nil
}

// PostChat sends a message along with conversation history to the specified Ollama model and returns the generated response.
func (s *Service) PostChat(ctx context.Context, modelName string, history []HistoryMessage, thinking bool) ([]byte, error) {
	systemPrompt := HistoryMessage{
		Role:    "system",
		Content: s.systemPrompt,
	}

	fullHistory := []HistoryMessage{systemPrompt}
	fullHistory = append(fullHistory, history...)
	body := Message{
		Model:     modelName,
		Messages:  fullHistory,
		Think:     thinking,
		KeepAlive: "5m",
		Stream:    false,
		Options: Options{
			Temperature: 0.8,
		},
	}

	jsonBody, err := json.Marshal(&body)
	if err != nil {
		return nil, fmt.Errorf("post chat: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildPath("/chat"), bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("post chat: %w", err)
	}

	client := http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post chat: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		scanner := bufio.NewScanner(response.Body)
		var fullAnswer []byte
		for scanner.Scan() {
			line := scanner.Bytes()
			var answerChunk MessageResponse
			if err := json.Unmarshal(line, &answerChunk); err != nil {
				return nil, fmt.Errorf("post chat: %w", err)
			}
			fullAnswer = append(fullAnswer, []byte(answerChunk.Response.Content)...)

			if answerChunk.Done {
				break
			}
		}

		return fullAnswer, nil
	}
	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("post chat: %w", err)
	}
	return nil, fmt.Errorf("post chat: %w", errors.New(string(b)))
}

// GenerateConversationTitle asks the smallest available AI model to generate a short title for a given message.
func (s *Service) GenerateConversationTitle(ctx context.Context, message, model string) (string, error) {
	titlePrompt := fmt.Sprintf("Can you generate a small conversation title for this message. Message: %s", message)

	history := []HistoryMessage{HistoryMessage{Role: "user", Content: titlePrompt}}

	answer, err := s.PostChat(ctx, model, history, false)
	if err != nil {
		return "", fmt.Errorf("error while generating title: %w", err)
	}

	if len(answer) == 0 {
		return "", fmt.Errorf("error while generating title: %w", errors.New("empty response when requested for a title"))
	}
	return string(answer), nil
}

// buildPath constructs a full URL for the Ollama API endpoints.
func buildPath(path string) string {
	return fmt.Sprintf("%s%s", ollamaBasePath, path)
}
