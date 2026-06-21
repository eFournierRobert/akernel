package ollama

import (
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

func TestBuildPath(t *testing.T) {
	assert.Equal(t, "http://localhost:11434/api/tags", buildPath("/tags"))
	assert.Equal(t, "http://localhost:11434/api/chat", buildPath("/chat"))
}

func TestNew(t *testing.T) {
	svc := New("test system prompt")
	assert.Equal(t, "system", svc.systemPrompt.Role)
	assert.Equal(t, "test system prompt", svc.systemPrompt.Content)
}

func TestGetModels_Success(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Models{
			ModelList: []Model{
				{Name: "llama3.2"},
				{Name: "gemma"},
			},
		})
	})

	svc := New("system")
	models, err := svc.GetModels(context.Background())

	require.NoError(t, err)
	assert.Len(t, models.ModelList, 2)
	assert.Equal(t, "llama3.2", models.ModelList[0].Name)
	assert.Equal(t, "gemma", models.ModelList[1].Name)
}

func TestGetModels_Non200(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error")
	})

	svc := New("system")
	_, err := svc.GetModels(context.Background())
	assert.Error(t, err)
}

func TestGetModels_BadJSON(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{invalid json")
	})

	svc := New("system")
	_, err := svc.GetModels(context.Background())
	assert.Error(t, err)
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

	svc := New("you are helpful")
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

	svc := New("you are helpful")
	_, err := svc.PostChat(context.Background(), "llama3.2", []HistoryMessage{
		{Role: "user", Content: "hello"},
	}, true)

	require.NoError(t, err)

	assert.Equal(t, "llama3.2", receivedBody.Model)
	assert.Len(t, receivedBody.Messages, 2)
	assert.Equal(t, "system", receivedBody.Messages[0].Role)
	assert.Equal(t, "you are helpful", receivedBody.Messages[0].Content)
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

	svc := New("system")
	_, err := svc.PostChat(context.Background(), "llama3.2", nil, false)
	require.NoError(t, err)
	assert.False(t, receivedBody.Think)
}

func TestPostChat_Non200(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "model not found")
	})

	svc := New("system")
	_, err := svc.PostChat(context.Background(), "nonexistent", nil, false)
	assert.Error(t, err)
}

func TestPostChat_BadJSONOn200(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not json")
	})

	svc := New("system")
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

	svc := New("system")
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

	svc := New("system")
	_, err := svc.GenerateConversationTitle(context.Background(), "hello world", "llama3.2")
	assert.Error(t, err)
}

func TestGenerateConversationTitle_PostChatFails(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server error")
	})

	svc := New("system")
	_, err := svc.GenerateConversationTitle(context.Background(), "hello world", "llama3.2")
	assert.Error(t, err)
}