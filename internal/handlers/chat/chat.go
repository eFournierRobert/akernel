package chat

import (
	"encoding/json"
	"io"
	"locassist/internal/database"
	"locassist/internal/handlers/dto"
	"locassist/internal/ollama"
	"locassist/internal/service"
	"net/http"
)

type Handler struct {
	OllamaService *ollama.Service
	Service       *service.ChatService
}

func New(ollamaService *ollama.Service, database *database.Database) *Handler {
	chatService := service.NewChatService(database)

	return &Handler{
		OllamaService: ollamaService,
		Service:       chatService,
	}
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
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

	conversationId, err := h.Service.InsertPrompt(&prompt)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	history, err := h.Service.FetchConversationHistory(conversationId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	answer, err := h.OllamaService.PostChat(prompt.Message, prompt.Model, history)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = h.Service.InsertAssistantAnswer(answer, conversationId); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var rawAnswer ollama.MessageFormat
	if err = json.Unmarshal(answer, &rawAnswer); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	answerDto := dto.Answer{
		Message:        rawAnswer,
		ConversationId: conversationId,
	}

	jsonResponse, err := json.Marshal(answerDto)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func (h *Handler) GetConversations(w http.ResponseWriter, r *http.Request) {
	conversations, err := h.Service.FetchConversations()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	convoJson, err := json.Marshal(conversations)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(convoJson)
}
