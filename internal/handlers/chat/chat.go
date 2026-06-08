package chat

import (
	"akernel/internal/database"
	"akernel/internal/handlers/dto"
	"akernel/internal/ollama"
	"akernel/internal/service"
	"encoding/json"
	"io"
	"log"
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
		log.Printf("ERROR: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	history, err := h.Service.FetchConversationHistory(conversationId)
	if err != nil {
		log.Printf("ERROR: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	answer, err := h.OllamaService.PostChat(prompt.Message, prompt.Model, history)
	if err != nil {
		log.Printf("ERROR: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var rawAnswer ollama.MessageFormat
	if err = json.Unmarshal(answer, &rawAnswer); err != nil {
		log.Printf("ERROR: %s", err.Error())
		w.WriteHeader(http.StatusOK)
		return
	}

	if len(rawAnswer.MessageBody) != 0 && rawAnswer.MessageBody[0].Body != "" {
		if err = h.Service.InsertAssistantAnswer(answer, conversationId); err != nil {
			log.Printf("ERROR: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		if err = h.Service.PopLastUserMessage(conversationId); err != nil {
			log.Printf("ERROR: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	answerDto := dto.Answer{
		Message:        rawAnswer,
		ConversationId: conversationId,
	}

	jsonResponse, err := json.Marshal(answerDto)
	if err != nil {
		log.Printf("ERROR: %s", err.Error())
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

func (h *Handler) GetModels(w http.ResponseWriter, r *http.Request) {
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

func convertSizeToGB(bytes int) float32 {
	return float32(bytes) / float32(1073741824)
}
