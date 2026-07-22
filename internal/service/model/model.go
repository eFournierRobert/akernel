// Package model implements the business logic for retrieving configured AI models.
package model

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/service"
	"context"
)

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
