package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"

	"github.com/adrg/xdg"
	"github.com/google/jsonschema-go/jsonschema"
)

const ollamaBasePath = "http://localhost:11434/api"

type Service struct {
	systemPrompt string
}

func New() (*Service, error) {
	systemPromptPath, err := xdg.ConfigFile("akernel/system.md")
	if err != nil {
		return nil, err
	}

	f, err := os.Open(systemPromptPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	systemPrompt, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	ollamaService := Service{
		systemPrompt: string(systemPrompt),
	}

	return &ollamaService, nil
}

func (oService *Service) GetModels() (*Models, error) {
	res, err := http.Get(buildPath("/tags"))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var models Models
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &models)
	if err != nil {
		return nil, err
	}

	return &models, nil
}

func (oService *Service) PostChat(message string, modelName string, history []HistoryMessage) ([]byte, error) {
	systemPrompt := HistoryMessage{
		Role:    "system",
		Content: oService.systemPrompt,
	}
	historyMessage := HistoryMessage{
		Role:    "user",
		Content: message,
	}

	history = append(history, systemPrompt)
	history = append(history, historyMessage)

	schema, err := jsonschema.For[MessageFormat](&jsonschema.ForOptions{})
	body := Message{
		Model:     modelName,
		Messages:  history,
		Format:    schema,
		Think:     true,
		KeepAlive: "5m",
		Stream:    true,
	}

	jsonBody, err := json.Marshal(&body)
	if err != nil {
		return nil, err
	}

	response, err := http.Post(buildPath("/chat"), "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		scanner := bufio.NewScanner(response.Body)
		var fullAnswer []byte
		for scanner.Scan() {
			line := scanner.Bytes()
			var answerChunk MessageResponse
			if err := json.Unmarshal(line, &answerChunk); err != nil {
				return nil, err
			}
			fullAnswer = append(fullAnswer, []byte(answerChunk.Response.Content)...)

			if answerChunk.Done {
				break
			}
		}

		return fullAnswer, nil
	}
	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return nil, errors.New(string(b))
}

func (oService *Service) GenerateConversationTitle(message string) (string, error) {
	models, err := oService.GetModels()
	if err != nil {
		return "", err
	}
	sort.Slice(models.ModelList, func(i, j int) bool {
		return models.ModelList[i].Size < models.ModelList[j].Size
	})

	titlePrompt := fmt.Sprintf("Can you generate a small conversation title of maximum five words for this message. Only answer with the title. Message: %s", message)

	var emptyHistory []HistoryMessage
	answerBytes, err := oService.PostChat(titlePrompt, models.ModelList[0].Model, emptyHistory)
	if err != nil {
		return "", err
	}

	var answer MessageFormat
	if err = json.Unmarshal(answerBytes, &answer); err != nil {
		return "", err
	}

	return answer.MessageBody[0].Body, nil
}

func buildPath(path string) string {
	return fmt.Sprintf("%s%s", ollamaBasePath, path)
}
