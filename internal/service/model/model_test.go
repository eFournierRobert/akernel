package model

import (
	"akernel/internal/config"
	"akernel/internal/service/inference"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestGetModels_EmptyList(t *testing.T) {
	svc := Service{
		inference: &mockInferenceClient{
			models: []config.Model{},
		},
	}

	result, err := svc.GetModels(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetModels_SingleModel(t *testing.T) {
	svc := Service{
		inference: &mockInferenceClient{
			models: []config.Model{
				{Name: "llama3.2", Description: "A small model"},
			},
		},
	}

	result, err := svc.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "llama3.2", result[0].Name)
	assert.Equal(t, "A small model", result[0].Description)
}

func TestGetModels_MultipleModels(t *testing.T) {
	svc := Service{
		inference: &mockInferenceClient{
			models: []config.Model{
				{Name: "small", Description: "Small"},
				{Name: "medium", Description: "Medium"},
				{Name: "large", Description: "Large"},
			},
		},
	}

	result, err := svc.GetModels(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 3)
}
