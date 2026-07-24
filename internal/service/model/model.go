// Package model implements the business logic for retrieving configured AI models.
package model

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/service/inference"
	"context"
)

// Service is the service for AI model-related actions.
type Service struct {
	inference inference.InferenceClient
}

// New creates a new Service that includes the given inference service.
func New(inference inference.InferenceClient) Service {
	return Service{
		inference: inference,
	}
}

// GetModels returns a list of available AI models.
func (s *Service) GetModels(ctx context.Context) ([]dto.Model, error) {
	modelList := s.inference.GetModels()

	var dtoModelList []dto.Model
	for _, model := range modelList {
		dtoModelList = append(dtoModelList, dto.Model{
			Name:        model.Name,
			Description: model.Description,
		})
	}

	return dtoModelList, nil
}
