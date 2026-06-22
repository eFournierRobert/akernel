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
	models *ollama.Models
	err    error
}

func (m *mockOllamaClient) GetModels(_ context.Context) (*ollama.Models, error) {
	return m.models, m.err
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
			models: &ollama.Models{ModelList: []ollama.Model{
				{Name: "llama3.2", Model: "llama3.2:latest", Size: 1073741824},
				{Name: "gemma2", Model: "gemma2:27b", Size: 2914605824},
			}},
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
			models: &ollama.Models{ModelList: []ollama.Model{}},
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

func TestGetModels_ServiceError(t *testing.T) {
	h := Handler{
		service: modelservice.New(&mockOllamaClient{
			err: assert.AnError,
		}),
	}
	mux := newTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func decodeBody(r interface{ String() string }, target interface{}) error {
	decoder := json.NewDecoder(strings.NewReader(r.String()))
	return decoder.Decode(target)
}
