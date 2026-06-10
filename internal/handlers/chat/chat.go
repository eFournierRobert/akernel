package chat

import (
	"akernel/internal/database"
	"akernel/internal/handlers/dto"
	"akernel/internal/service"
	"akernel/internal/service/chat"
	"akernel/internal/service/ollama"
	"encoding/json"
	"errors"
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
		log.Println(err)
		switch {
		case errors.Is(err, service.ErrInvalidConversation), errors.Is(err, service.ErrModelNotFound):
			w.WriteHeader(http.StatusBadRequest)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
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
		log.Println(err)
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

// GetModels retrieves a list of available AI models.
func (h *Handler) GetModels(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	modelList, err := h.service.GetModels()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	modelsJson, err := json.Marshal(modelList)
	if err != nil {
		fmt.Errorf("error: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(modelsJson)
}
