package model

import (
	"akernel/internal/config"
	"akernel/internal/service/inference"
	modelservice "akernel/internal/service/model"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"akernel/internal/handlers/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMux(handler Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /models", handler.GetModels)
	return mux
}

type mockInferenceClient struct {
	models []config.Model
}

func (m *mockInferenceClient) GetModels() []config.Model {
	return m.models
}

func (m *mockInferenceClient) PostChat(_ context.Context, _ string, _ []inference.HistoryMessage, _ bool) (inference.MessageResponse, error) {
	return inference.MessageResponse{}, nil
}

func (m *mockInferenceClient) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func TestGetModels_SuccessWithResults(t *testing.T) {
	h := Handler{
		service: modelservice.New(&mockInferenceClient{
			models: []config.Model{
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
		service: modelservice.New(&mockInferenceClient{
			models: []config.Model{},
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

func decodeBody(r io.Reader, target interface{}) error {
	return json.NewDecoder(r).Decode(target)
}
