package model

import (
	"akernel/internal/service/ollama"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestGetModels_EmptyList(t *testing.T) {
	svc := New(&mockOllamaClient{
		models: &ollama.Models{ModelList: []ollama.Model{}},
	})

	result, err := svc.GetModels(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetModels_SingleModel(t *testing.T) {
	svc := New(&mockOllamaClient{
		models: &ollama.Models{ModelList: []ollama.Model{
			{Name: "llama3.2", Model: "llama3.2:latest", Size: 1073741824},
		}},
	})

	result, err := svc.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "llama3.2", result[0].Name)
	assert.Equal(t, "llama3.2:latest", result[0].Model)
	assert.Equal(t, float32(1.0), result[0].Size)
}

func TestGetModels_MultipleModels(t *testing.T) {
	svc := New(&mockOllamaClient{
		models: &ollama.Models{ModelList: []ollama.Model{
			{Name: "small", Model: "small", Size: 536870912},
			{Name: "medium", Model: "medium", Size: bytesPerGB},
			{Name: "large", Model: "large", Size: 5 * bytesPerGB},
		}},
	})

	result, err := svc.GetModels(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, float32(0.5), result[0].Size)
	assert.Equal(t, float32(1.0), result[1].Size)
	assert.Equal(t, float32(5.0), result[2].Size)
}

func TestGetModels_OllamaError(t *testing.T) {
	svc := New(&mockOllamaClient{
		err: errors.New("connection refused"),
	})

	result, err := svc.GetModels(context.Background())
	assert.Error(t, err)
	assert.Nil(t, result)
}
