package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akernel/internal/handlers/dto"
	modelservice "akernel/internal/service/model"
	"akernel/internal/service/ollama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMux(handler Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /models", handler.GetModels)
	return mux
}

type mockOllamaClient struct {
	models []ollama.Model
}

func (m *mockOllamaClient) GetModels() []ollama.Model {
	return m.models
}

func (m *mockOllamaClient) GetModelId(_ string) string {
	return ""
}

func (m *mockOllamaClient) PostChat(_ context.Context, _ string, _ []ollama.HistoryMessage, _ bool) (ollama.MessageResponse, error) {
	return ollama.MessageResponse{}, nil
}

func (m *mockOllamaClient) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func TestGetModels_SuccessWithResults(t *testing.T) {
	h := Handler{
		service: modelservice.New(&mockOllamaClient{
			models: []ollama.Model{
				{Name: "llama3.2", Description: "A small model"},
				{Name: "gemma2", Description: "A larger model"},
			},
		}),
	}
	mux := newTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var result []dto.Model
	require.NoError(t, decodeBody(rec.Body, &result))
	require.Len(t, result, 2)
	assert.Equal(t, "llama3.2", result[0].Name)
	assert.Equal(t, "gemma2", result[1].Name)
}

func TestGetModels_EmptyList(t *testing.T) {
	h := Handler{
		service: modelservice.New(&mockOllamaClient{
			models: []ollama.Model{},
		}),
	}
	mux := newTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result []dto.Model
	require.NoError(t, decodeBody(rec.Body, &result))
	assert.Empty(t, result)
}

func decodeBody(r interface{ String() string }, target interface{}) error {
	decoder := json.NewDecoder(strings.NewReader(r.String()))
	return decoder.Decode(target)
}
