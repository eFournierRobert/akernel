package chat

import (
	"akernel/internal/database"
	"akernel/internal/handlers/dto"
	"akernel/internal/service/chat"
	"akernel/internal/service/ollama"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Handler manages HTTP requests for chat operations
type Handler struct {
	service *chat.Service
}

// New creates a new instance of Handler with the required services.
func New(ollamaService *ollama.Service, database *database.Database) *Handler {
	chatService := chat.NewChatService(database, ollamaService)

	return &Handler{
		service: chatService,
	}
}

// Chat handles POST requests to process a user prompt and return an AI-generated response.
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	bodyContent, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var prompt dto.Prompt
	if err = json.Unmarshal(bodyContent, &prompt); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	answer, err := h.service.GeneratePromptAnswer(&prompt)
	if err != nil {
		fmt.Errorf("ERROR: %w", err)
		w.WriteHeader(http.StatusInternalServerError) // temporary
		return
	}

	jsonResponse, err := json.Marshal(answer)
	if err != nil {
		fmt.Errorf("ERROR: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

// GetConversations retrieves a list of all available conversations for the user.
func (h *Handler) GetConversations(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	conversations, err := h.service.FetchConversations()
	if err != nil {
		log.Printf("ERROR: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	convoJson, err := json.Marshal(conversations)
	if err != nil {
		log.Printf("ERROR: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(convoJson)
}

// GetModels retrieves a list of available AI models from the Ollama service.
func (h *Handler) GetModels(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	modelList, err := h.OllamaService.GetModels()
	if err != nil {
		log.Printf("ERROR: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var models []dto.Model
	for _, model := range modelList.ModelList {
		models = append(models, dto.Model{
			Name:  model.Name,
			Model: model.Model,
			Size:  convertSizeToGB(model.Size),
		})
	}

	modelsJson, err := json.Marshal(models)
	if err != nil {
		log.Printf("ERROR: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(modelsJson)
}

// convertSizeToGB converts a model size from bytes to gigabytes.
func convertSizeToGB(bytes int) float32 {
	return float32(bytes) / float32(1073741824)
}
