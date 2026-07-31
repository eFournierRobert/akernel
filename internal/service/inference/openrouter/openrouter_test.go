package openrouter

import (
	"akernel/internal/config"
	"akernel/internal/service/inference/tools"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() config.Config {
	return config.Config{
		OpenRouterApiKey: "test-api-key",
		SystemPrompt:     "You are a helpful assistant.",
		Models:           []config.Model{},
	}
}

func setupTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldPath := openrouterBasePath
	openrouterBasePath = server.URL
	t.Cleanup(func() { openrouterBasePath = oldPath })
}

func TestNew(t *testing.T) {
	cfg := testConfig()
	svc := New(cfg)

	assert.Equal(t, cfg.OpenRouterApiKey, svc.config.OpenRouterApiKey)
	assert.Equal(t, cfg.SystemPrompt, svc.config.SystemPrompt)
}

func TestBuildPath(t *testing.T) {
	result := buildPath("/chat/completions")
	assert.Equal(t, "https://openrouter.ai/api/v1/chat/completions", result)
}

func TestPostChat_Success(t *testing.T) {
	responseBody := MessageResponse{
		Choices: []struct {
			Message HistoryMessage `json:"message"`
		}{
			{Message: HistoryMessage{Role: "assistant", Content: toPtr("Hello back!")}},
		},
	}

	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body Message
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "my-model", body.Model)
		assert.False(t, body.Stream)
		assert.NotEmpty(t, body.Tools)
		assert.Len(t, body.Messages, 1)
		assert.Equal(t, "user", body.Messages[0].Role)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responseBody)
	})

	svc := New(testConfig())
	resp, err := svc.PostChat(context.Background(), "my-model", []HistoryMessage{
		{Role: "user", Content: toPtr("Hello")},
	})

	require.NoError(t, err)
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "assistant", resp.Choices[0].Message.Role)
	assert.Equal(t, "Hello back!", *resp.Choices[0].Message.Content)
}

func TestPostChat_Non200Response(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, "{\"error\": \"invalid api key\"}")
	})

	svc := New(testConfig())
	_, err := svc.PostChat(context.Background(), "my-model", nil)

	assert.Error(t, err)
}

func TestPostChat_MalformedJSONResponse(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "{not valid json")
	})

	svc := New(testConfig())
	_, err := svc.PostChat(context.Background(), "my-model", nil)

	assert.Error(t, err)
}

func TestPostChat_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	svc := New(testConfig())
	_, err := svc.PostChat(ctx, "my-model", nil)

	assert.Error(t, err)
}

func TestPostChat_HTTPClientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	addr := srv.URL
	srv.Close()

	oldPath := openrouterBasePath
	openrouterBasePath = addr
	defer func() { openrouterBasePath = oldPath }()

	svc := New(testConfig())
	_, err := svc.PostChat(context.Background(), "my-model", nil)
	assert.Error(t, err)
}

func TestPostChat_ToolSchemasIncluded(t *testing.T) {
	var capturedBody Message
	responseBody := MessageResponse{
		Choices: []struct {
			Message HistoryMessage `json:"message"`
		}{},
	}

	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responseBody)
	})

	svc := New(testConfig())
	_, err := svc.PostChat(context.Background(), "my-model", nil)

	require.NoError(t, err)
	assert.Equal(t, len(tools.GetAllToolSchemas()), len(capturedBody.Tools))
}

func TestPostChat_WithTokenUsage(t *testing.T) {
	responseBody := MessageResponse{
		Choices: []struct {
			Message HistoryMessage `json:"message"`
		}{
			{Message: HistoryMessage{Role: "assistant", Content: toPtr("Done!")}},
		},
		Usage: struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		}{PromptTokens: 15, TotalTokens: 42},
	}

	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responseBody)
	})

	svc := New(testConfig())
	resp, err := svc.PostChat(context.Background(), "my-model", nil)

	require.NoError(t, err)
	assert.Equal(t, 42, resp.Usage.TotalTokens)
	assert.Equal(t, 15, resp.Usage.PromptTokens)
}

func TestPostChat_EmptyHistory(t *testing.T) {
	responseBody := MessageResponse{
		Choices: []struct {
			Message HistoryMessage `json:"message"`
		}{
			{Message: HistoryMessage{Role: "assistant", Content: toPtr("Hi!")}},
		},
	}

	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responseBody)
	})

	svc := New(testConfig())
	resp, err := svc.PostChat(context.Background(), "my-model", nil)

	require.NoError(t, err)
	assert.Len(t, resp.Choices, 1)
}

func TestPostChat_503ServiceUnavailable(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, "rate limited")
	})

	svc := New(testConfig())
	_, err := svc.PostChat(context.Background(), "my-model", nil)

	assert.Error(t, err)
}

func TestPostChat_NilApiKey(t *testing.T) {
	cfg := testConfig()
	cfg.OpenRouterApiKey = ""
	svc := New(cfg)

	responseBody := MessageResponse{
		Choices: []struct {
			Message HistoryMessage `json:"message"`
		}{},
	}

	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responseBody)
	})

	_, err := svc.PostChat(context.Background(), "my-model", nil)

	require.NoError(t, err)
}

func toPtr(s string) *string {
	return &s
}

