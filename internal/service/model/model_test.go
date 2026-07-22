package model

import (
	"akernel/internal/service/ollama"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestGetModels_EmptyList(t *testing.T) {
	svc := New(&mockOllamaClient{
		models: []ollama.Model{},
	})

	result, err := svc.GetModels(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetModels_SingleModel(t *testing.T) {
	svc := New(&mockOllamaClient{
		models: []ollama.Model{
			{Name: "llama3.2", Description: "A small model"},
		},
	})

	result, err := svc.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "llama3.2", result[0].Name)
	assert.Equal(t, "A small model", result[0].Description)
}

func TestGetModels_MultipleModels(t *testing.T) {
	svc := New(&mockOllamaClient{
		models: []ollama.Model{
			{Name: "small", Description: "Small"},
			{Name: "medium", Description: "Medium"},
			{Name: "large", Description: "Large"},
		},
	})

	result, err := svc.GetModels(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 3)
}
