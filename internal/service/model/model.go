// Package model implements the business logic for retrieving AI models from Ollama.
package model

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/service"
	"context"
)

// bytesPerGB is how many bytes is in a gigabyte.
const bytesPerGB = 1073741824

// Service is the service for AI model-related actions.
type Service struct {
	ollama service.OllamaClient
}

// New creates a new Service that includes the given Ollama service.
func New(ollama service.OllamaClient) Service {
	return Service{
		ollama,
	}
}

// GetModels returns a list of available AI models.
func (s *Service) GetModels(ctx context.Context) ([]dto.Model, error) {
	modelList := s.ollama.GetModels()

	var dtoModelList []dto.Model
	for _, model := range modelList {
		dtoModelList = append(dtoModelList, dto.Model{
			Name:        model.Name,
			Description: model.Description,
		})
	}

	return dtoModelList, nil
}
