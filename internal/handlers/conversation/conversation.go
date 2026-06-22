// Package conversation provides the HTTP handler for conversation-related endpoints.
package conversation

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/service"
	"akernel/internal/service/conversation"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/jmoiron/sqlx"
)

// Handler is the handler for conversation-related actions.
type Handler struct {
	service conversation.Service
}

// New creates a new working Handler.
func New(db *sqlx.DB, ollamaService service.OllamaClient) Handler {
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

	convoId, err := h.getConversationId(w, r)
	if err != nil {
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

// PostSummarize generates a summary for a conversation identified by ID from the path
// using the model name provided in the request body.
func (h *Handler) PostSummarize(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	convoId, err := h.getConversationId(w, r)
	if err != nil {
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

// getConversationId extracts and validates the conversation ID from the request path.
// It returns a 400 status and non-nil error if the ID is missing, non-numeric, or less than 1.
func (h *Handler) getConversationId(w http.ResponseWriter, r *http.Request) (int, error) {
	convoIdStr := r.PathValue("id")
	convoId, err := strconv.Atoi(convoIdStr)
	if err != nil || convoId < 1 {
		r.Body.Close()
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return -1, errors.New("invalid id")
	}

	return convoId, nil
}
