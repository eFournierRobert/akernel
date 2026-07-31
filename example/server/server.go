package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var baseUrl string = "http://localhost:50500"

func ListModels() ([]Model, error) {
	resp, err := http.Get(buildUrl("/models"))
	if err != nil || resp.StatusCode != 200 {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var models []Model
	if err = json.Unmarshal(body, &models); err != nil {
		return nil, err
	}

	return models, nil
}

func ListConversations() ([]Conversation, error) {
	resp, err := http.Get(buildUrl("/conversation"))
	if err != nil || resp.StatusCode != 200 {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var models []Conversation
	if err = json.Unmarshal(body, &models); err != nil {
		return nil, err
	}

	return models, nil
}

func PostChat(conversationId int, model, message string) ([]ChatResponse, error) {
	chat := ChatBody{
		Model:          model,
		Message:        message,
		ConversationId: conversationId,
	}

	body, err := json.Marshal(chat)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(buildUrl("/chat"), http.DetectContentType(body), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, errors.New("conversation or model was not found")
	}
	if resp.StatusCode == 500 {
		return nil, errors.New("internal server error while treating the request")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var chatResponses []ChatResponse
	if err = json.Unmarshal(respBody, &chatResponses); err != nil {
		return nil, err
	}

	return chatResponses, nil
}

func buildUrl(endpoint string) string {
	return fmt.Sprintf("%s%s", baseUrl, endpoint)
}
