package ollama

import (
	"akernel/internal/config"
	"akernel/internal/service/ollama/tools"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldPath := ollamaBasePath
	ollamaBasePath = server.URL
	t.Cleanup(func() { ollamaBasePath = oldPath })

	return server
}

func newWithModels() *Service {
	return New("system", []config.Model{
		{Nickname: "llama3.2", Id: "llama3.2:latest", Description: "A small model"},
		{Nickname: "gemma", Id: "gemma:27b", Description: "A larger model"},
	})
}

func TestBuildPath(t *testing.T) {
	assert.Equal(t, "http://localhost:11434/api/tags", buildPath("/tags"))
	assert.Equal(t, "http://localhost:11434/api/chat", buildPath("/chat"))
}

func TestNew(t *testing.T) {
	svc := New("test system prompt", []config.Model{})
	assert.Equal(t, "system", svc.systemPrompt.Role)
	assert.Equal(t, "test system prompt", svc.systemPrompt.Content)
	assert.Empty(t, svc.models)
}

func TestNew_ModelConversion(t *testing.T) {
	svc := New("system", []config.Model{
		{Nickname: "small", Id: "llama3.2:latest", Description: "A small model"},
	})

	require.Len(t, svc.models, 1)
	assert.Equal(t, "small", svc.models[0].Name)
	assert.Equal(t, "llama3.2:latest", svc.models[0].RealName)
	assert.Equal(t, "A small model", svc.models[0].Description)
}

func TestGetModels(t *testing.T) {
	svc := newWithModels()
	models := svc.GetModels()

	assert.Len(t, models, 2)
	assert.Equal(t, "llama3.2", models[0].Name)
	assert.Equal(t, "gemma", models[1].Name)
}

func TestGetModelId_Found(t *testing.T) {
	svc := newWithModels()

	id := svc.GetModelId("llama3.2")
	assert.Equal(t, "llama3.2:latest", id)

	id = svc.GetModelId("gemma")
	assert.Equal(t, "gemma:27b", id)
}

func TestGetModelId_NotFound(t *testing.T) {
	svc := newWithModels()

	id := svc.GetModelId("nonexistent")
	assert.Equal(t, "", id)
}

func TestPostChat_Success(t *testing.T) {
	var receivedBody Message
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MessageResponse{
			Model:  "llama3.2",
			Done:   true,
			Response: HistoryMessage{
				Role:    "assistant",
				Content: "hello back",
			},
			TokenCount:       10,
			PromptTokenCount: 5,
		})
	})

	svc := newWithModels()
	resp, err := svc.PostChat(context.Background(), "llama3.2", []HistoryMessage{
		{Role: "user", Content: "hello"},
	}, false)

	require.NoError(t, err)
	assert.Equal(t, "hello back", resp.Response.Content)
	assert.Equal(t, 10, resp.TokenCount)
}

func TestPostChat_RequestShape(t *testing.T) {
	var receivedBody Message
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(MessageResponse{Done: true})
	})

	svc := newWithModels()
	_, err := svc.PostChat(context.Background(), "llama3.2", []HistoryMessage{
		{Role: "user", Content: "hello"},
	}, true)

	require.NoError(t, err)

	assert.Equal(t, "llama3.2", receivedBody.Model)
	assert.Len(t, receivedBody.Messages, 2)
	assert.Equal(t, "system", receivedBody.Messages[0].Role)
	assert.Equal(t, "system", receivedBody.Messages[0].Content)
	assert.Equal(t, "user", receivedBody.Messages[1].Role)
	assert.Equal(t, "hello", receivedBody.Messages[1].Content)

	assert.True(t, receivedBody.Think)
	assert.False(t, receivedBody.Stream)
	assert.Equal(t, "5m", receivedBody.KeepAlive)
	assert.Equal(t, float32(0.8), receivedBody.Options.Temperature)
	assert.Equal(t, 32000, receivedBody.Options.NumCtx)
	assert.Len(t, receivedBody.Tools, len(tools.GetAllToolSchemas()))
}

func TestPostChat_ThinkingFalse(t *testing.T) {
	var receivedBody Message
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(MessageResponse{Done: true})
	})

	svc := newWithModels()
	_, err := svc.PostChat(context.Background(), "llama3.2", nil, false)
	require.NoError(t, err)
	assert.False(t, receivedBody.Think)
}

func TestPostChat_Non200(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "model not found")
	})

	svc := newWithModels()
	_, err := svc.PostChat(context.Background(), "nonexistent", nil, false)
	assert.Error(t, err)
}

func TestPostChat_BadJSONOn200(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not json")
	})

	svc := newWithModels()
	_, err := svc.PostChat(context.Background(), "llama3.2", nil, false)
	assert.Error(t, err)
}

func TestGenerateConversationTitle_Success(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MessageResponse{
			Done: true,
			Response: HistoryMessage{
				Role:    "assistant",
				Content: "My Title",
			},
		})
	})

	svc := newWithModels()
	title, err := svc.GenerateConversationTitle(context.Background(), "hello world", "llama3.2")

	require.NoError(t, err)
	assert.Equal(t, "My Title", title)
}

func TestGenerateConversationTitle_EmptyResponse(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MessageResponse{
			Done: true,
			Response: HistoryMessage{
				Role:    "assistant",
				Content: "",
			},
		})
	})

	svc := newWithModels()
	_, err := svc.GenerateConversationTitle(context.Background(), "hello world", "llama3.2")
	assert.Error(t, err)
}

func TestGenerateConversationTitle_PostChatFails(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server error")
	})

	svc := newWithModels()
	_, err := svc.GenerateConversationTitle(context.Background(), "hello world", "llama3.2")
	assert.Error(t, err)
}
