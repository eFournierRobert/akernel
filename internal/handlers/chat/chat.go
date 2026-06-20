// Package chat provides the HTTP handler for chat-related endpoints.
package chat

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/service"
	"akernel/internal/service/chat"
	"akernel/internal/service/ollama"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/jmoiron/sqlx"
)

// Handler manages HTTP requests for chat operations
type Handler struct {
	service *chat.Service
}

// New creates a new instance of Handler with the required services.
func New(ollamaService ollama.Service, database *sqlx.DB) *Handler {
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

	answer, err := h.service.GeneratePromptAnswer(r.Context(), &prompt)
	if err != nil {
		log.Println(err)
		switch {
		case errors.Is(err, service.ErrInvalidConversation), errors.Is(err, service.ErrModelNotFound):
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	jsonResponse, err := json.Marshal(answer)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

// GetFullConversation gets the full conversation and returns it in JSON.
func (h *Handler) GetFullConversation(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	convoIdStr := r.PathValue("id")
	convoId, err := strconv.Atoi(convoIdStr)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	messages, err := h.service.GetConversation(convoId)
	if err != nil {
		log.Println(err)
		if errors.Is(err, service.ErrConversationDoesNotExist) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	jsonMessages, err := json.Marshal(messages)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonMessages)
}
