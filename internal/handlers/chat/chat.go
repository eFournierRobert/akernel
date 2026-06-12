package chat

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/repository"
	"akernel/internal/service"
	"akernel/internal/service/chat"
	"akernel/internal/service/ollama"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
)

// Handler manages HTTP requests for chat operations
type Handler struct {
	service *chat.Service
}

// New creates a new instance of Handler with the required services.
func New(ollamaService *ollama.Service, database *repository.Database) *Handler {
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
			w.WriteHeader(http.StatusBadRequest)
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
		log.Println(err)
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

// DeleteConversation deletes a conversation and messages by its ID.
func (h *Handler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	convoIdStr := r.PathValue("id")
	convoId, err := strconv.Atoi(convoIdStr)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err = h.service.DeleteConversation(convoId); err != nil {
		log.Println(err)
		if errors.Is(err, service.ErrConversationDoesNotExist) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetConversation gets the full conversation and returns it in JSON.
func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
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
