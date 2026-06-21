// Package model implements the business logic for retrieving AI models from Ollama.
package model

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/service/ollama"
	"context"
)

// bytesPerGB is how many bytes is in a gigabyte.
const bytesPerGB = 1073741824

// OllamaClient abstracts the Ollama service for model retrieval, allowing mocking in tests.
type OllamaClient interface {
	GetModels(ctx context.Context) (*ollama.Models, error)
}

// Service is the service for AI model-related actions.
type Service struct {
	ollama OllamaClient
}

// New creates a new Service that includes the given Ollama service.
func New(ollama OllamaClient) Service {
	return Service{
		ollama,
	}
}

// GetModels returns a list of available AI models.
func (s *Service) GetModels(ctx context.Context) ([]dto.Model, error) {
	modelList, err := s.ollama.GetModels(ctx)
	if err != nil {
		return nil, err
	}

	var dtoModelList []dto.Model
	for _, model := range modelList.ModelList {
		dtoModelList = append(dtoModelList, dto.Model{
			Name:  model.Name,
			Model: model.Model,
			Size:  convertSizeToGB(model.Size),
		})
	}

	return dtoModelList, nil
}

// convertSizeToGB converts a model size from bytes to gigabytes.
func convertSizeToGB(bytes int) float32 {
	return float32(bytes) / float32(bytesPerGB)
}
