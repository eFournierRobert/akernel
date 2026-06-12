package model

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/service/ollama"
	"context"
)

type Service struct {
	ollama ollama.Service
}

func New(ollama ollama.Service) Service {
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
	return float32(bytes) / float32(1073741824)
}
