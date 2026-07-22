// Package model provides the HTTP handler for AI model-related endpoints.
package model

import (
	"akernel/internal/config"
	"akernel/internal/service/model"
	"encoding/json"
	"log"
	"net/http"
)

// Handler is the request handler for AI model-related actions.
type Handler struct {
	service model.Service
}

// New creates a new working Handler.
func New(config config.Config) Handler {
	return Handler{
		service: model.New(config),
	}
}

// GetModels retrieves a list of available AI models.
func (h *Handler) GetModels(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	modelList, err := h.service.GetModels(r.Context())
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	modelsJson, err := json.Marshal(modelList)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(modelsJson)
}
