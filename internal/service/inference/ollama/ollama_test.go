package ollama

import (
	"akernel/internal/service/inference/tools"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldPath := ollamaBasePath
	ollamaBasePath = server.URL
	t.Cleanup(func() { ollamaBasePath = oldPath })
}

func TestBuildPath(t *testing.T) {
	assert.Equal(t, "http://localhost:11434/api/tags", buildPath("/tags"))
	assert.Equal(t, "http://localhost:11434/api/chat", buildPath("/chat"))
}

func TestNew(t *testing.T) {
	svc := New()
	assert.NotNil(t, &svc)
}

func TestPostChat_Success(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MessageResponse{
			Model:  "llama3.2:latest",
			Done:   true,
			Response: HistoryMessage{
				Role:    "assistant",
				Content: "hello back",
			},
			TokenCount:       10,
			PromptTokenCount: 5,
		})
	})

	svc := New()
	resp, err := svc.PostChat(context.Background(), "llama3.2:latest", []HistoryMessage{
		{Role: "user", Content: "hello"},
	}, false)

	require.NoError(t, err)
	assert.Equal(t, "hello back", resp.Response.Content)
	assert.Equal(t, 10, resp.TokenCount)
	assert.Equal(t, 5, resp.PromptTokenCount)
}

func TestPostChat_RequestShape(t *testing.T) {
	var receivedBody Message
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(MessageResponse{Done: true})
	})

	svc := New()
	_, err := svc.PostChat(context.Background(), "llama3.2:latest", []HistoryMessage{
		{Role: "user", Content: "hello"},
	}, true)

	require.NoError(t, err)

	assert.Equal(t, "llama3.2:latest", receivedBody.Model)
	assert.Len(t, receivedBody.Messages, 1)
	assert.Equal(t, "user", receivedBody.Messages[0].Role)
	assert.Equal(t, "hello", receivedBody.Messages[0].Content)

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

	svc := New()
	_, err := svc.PostChat(context.Background(), "llama3.2:latest", nil, false)
	require.NoError(t, err)
	assert.False(t, receivedBody.Think)
}

func TestPostChat_Non200(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "model not found")
	})

	svc := New()
	_, err := svc.PostChat(context.Background(), "nonexistent", nil, false)
	assert.Error(t, err)
}

func TestPostChat_BadJSONOn200(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not json")
	})

	svc := New()
	_, err := svc.PostChat(context.Background(), "llama3.2:latest", nil, false)
	assert.Error(t, err)
}

func TestPostChat_ToolCallsInResponse(t *testing.T) {
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MessageResponse{
			Done: true,
			Response: HistoryMessage{
				Role:    "assistant",
				Content: "",
				ToolCalls: []tools.ToolCall{
					{Function: tools.Function{Name: "get_current_time", Arguments: nil}},
				},
			},
			TokenCount: 8,
		})
	})

	svc := New()
	resp, err := svc.PostChat(context.Background(), "llama3.2:latest", []HistoryMessage{
		{Role: "user", Content: "What time is it?"},
	}, false)

	require.NoError(t, err)
	assert.Len(t, resp.Response.ToolCalls, 1)
	assert.Equal(t, "get_current_time", resp.Response.ToolCalls[0].Function.Name)
}

func TestPostChat_EmptyHistory(t *testing.T) {
	var receivedBody Message
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(MessageResponse{Done: true})
	})

	svc := New()
	_, err := svc.PostChat(context.Background(), "llama3.2:latest", nil, false)
	require.NoError(t, err)
	assert.Empty(t, receivedBody.Messages)
}

func TestPostChat_ContextCancellation(t *testing.T) {
	tickerInvoked := false
	setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		tickerInvoked = true
		<-r.Context().Done()
	})

	svc := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.PostChat(ctx, "llama3.2:latest", nil, false)
	assert.Error(t, err)
	assert.False(t, tickerInvoked)
}
