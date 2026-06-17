package conversation

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/service"
	"akernel/internal/service/conversation"
	"akernel/internal/service/ollama"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/jmoiron/sqlx"
)

type Handler struct {
	service conversation.Service
}

func New(db *sqlx.DB, ollamaService ollama.Service) Handler {
	return Handler{
		service: conversation.New(db, ollamaService),
	}
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

func (h *Handler) PostSummarize(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	convoIdStr := r.PathValue("id")
	convoId, err := strconv.Atoi(convoIdStr)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	bodyContent, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var model dto.ModelOnly
	if err = json.Unmarshal(bodyContent, &model); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err = h.service.CreateNewSummary(r.Context(), convoId, model.Name); err != nil {
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
