package model

import (
	"akernel/internal/config"
	"akernel/internal/handlers/dto"
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

func TestGetModels(t *testing.T) {
	testCases := []struct {
		name     string
		models   []config.Model
		expected []dto.Model
	}{
		{
			name:     "empty list",
			models:   []config.Model{},
			expected: nil,
		},
		{
			name: "single model",
			models: []config.Model{
				{Name: "llama3.2", Description: "A small model"},
			},
			expected: []dto.Model{
				{Name: "llama3.2", Description: "A small model"},
			},
		},
		{
			name: "multiple models",
			models: []config.Model{
				{Name: "small", Description: "Small"},
				{Name: "medium", Description: "Medium"},
				{Name: "large", Description: "Large"},
			},
			expected: []dto.Model{
				{Name: "small", Description: "Small"},
				{Name: "medium", Description: "Medium"},
				{Name: "large", Description: "Large"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := Service{
				inference: &mockInferenceClient{models: tc.models},
			}

			result, err := svc.GetModels(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
