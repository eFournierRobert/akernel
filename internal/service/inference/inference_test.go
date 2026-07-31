package inference

import (
	"akernel/internal/config"
	"akernel/internal/service/inference/ollama"
	"akernel/internal/service/inference/openrouter"
	"akernel/internal/service/inference/tools"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOllamaClient struct {
	response ollama.MessageResponse
	err      error
}

func (m *mockOllamaClient) PostChat(_ context.Context, _ string, _ []ollama.HistoryMessage, _ bool) (ollama.MessageResponse, error) {
	return m.response, m.err
}

type mockOpenRouterClient struct {
	response openrouter.MessageResponse
	err      error
}

func (m *mockOpenRouterClient) PostChat(_ context.Context, _ string, _ []openrouter.HistoryMessage) (openrouter.MessageResponse, error) {
	return m.response, m.err
}

func mockService(ollamaClient ollamaClient, models []config.Model, systemPrompt string) Service {
	return Service{
		systemPrompt: HistoryMessage{
			Role:    "system",
			Content: systemPrompt,
		},
		models:        models,
		ollamaService: ollamaClient,
	}
}

func testOllamaConfig() config.Config {
	return config.Config{
		SystemPrompt: "You are a helpful assistant.",
		Models: []config.Model{
			{Name: "llama3.2", Id: "llama3.2:latest", Description: "A small model", Provider: "ollama"},
			{Name: "gemma", Id: "gemma:27b", Description: "A larger model", Provider: "ollama"},
		},
	}
}

func testOpenRouterConfig() config.Config {
	return config.Config{
		SystemPrompt:     "You are a helpful assistant.",
		OpenRouterApiKey: "test-key",
		Models: []config.Model{
			{Name: "gpt4", Id: "openai/gpt-4", Description: "OpenRouter model", Provider: "openrouter"},
		},
	}
}

func toPtrOr(s string) *string {
	return &s
}

func TestNew(t *testing.T) {
	cfg := testOllamaConfig()
	svc := New(cfg)

	models := svc.GetModels()
	require.Len(t, models, 2)
	assert.Equal(t, "llama3.2", models[0].Name)
	assert.Equal(t, "gemma", models[1].Name)
}

func TestGetModel_Found(t *testing.T) {
	svc := mockService(nil, testOllamaConfig().Models, "system")

	model, err := svc.GetModel("llama3.2")
	require.NoError(t, err)
	assert.Equal(t, "llama3.2", model.Name)
	assert.Equal(t, "llama3.2:latest", model.Id)
}

func TestGetModel_NotFound(t *testing.T) {
	svc := mockService(nil, testOllamaConfig().Models, "system")

	_, err := svc.GetModel("nonexistent")
	require.Error(t, err)
}

func TestPostChat_Success(t *testing.T) {
	mock := &mockOllamaClient{
		response: ollama.MessageResponse{
			Model: "llama3.2:latest",
			Response: ollama.HistoryMessage{
				Role:     "assistant",
				Content:  "Hello back!",
				Thinking: "reasoning trace",
			},
			TokenCount:       10,
			PromptTokenCount: 5,
		},
	}

	svc := mockService(mock, testOllamaConfig().Models, "You are a helpful assistant.")
	resp, err := svc.PostChat(context.Background(), "llama3.2", []HistoryMessage{
		{Role: "user", Content: "Hello"},
	}, false)

	require.NoError(t, err)
	assert.Equal(t, "llama3.2", resp.Model)
	assert.Equal(t, "Hello back!", resp.Response.Content)
	assert.Equal(t, "assistant", resp.Response.Role)
	assert.Equal(t, "reasoning trace", resp.Response.Thinking)
	assert.Equal(t, 10, resp.TokenCount)
	assert.Equal(t, 5, resp.PromptTokenCount)
}

func TestPostChat_PassesModelIdNotName(t *testing.T) {
	var receivedModelID string

	testMock := &mockWithCapture{
		mockOllamaClient: &mockOllamaClient{response: ollama.MessageResponse{}},
		onCall: func(modelID string) {
			receivedModelID = modelID
		},
	}

	svc := mockService(testMock, testOllamaConfig().Models, "system")
	_, err := svc.PostChat(context.Background(), "gemma", nil, false)
	require.NoError(t, err)
	assert.Equal(t, "gemma:27b", receivedModelID)
}

func TestPostChat_PrependsSystemPrompt(t *testing.T) {
	var receivedHistory []ollama.HistoryMessage

	mock := &mockWithCapture{
		mockOllamaClient: &mockOllamaClient{response: ollama.MessageResponse{}},
		onCallAndHistory: func(_ string, history []ollama.HistoryMessage) {
			receivedHistory = history
		},
	}

	svc := mockService(mock, testOllamaConfig().Models, "You are a helpful assistant.")
	_, err := svc.PostChat(context.Background(), "llama3.2", []HistoryMessage{
		{Role: "user", Content: "Hello"},
	}, false)

	require.NoError(t, err)
	require.Len(t, receivedHistory, 2)
	assert.Equal(t, "system", receivedHistory[0].Role)
	assert.Equal(t, "You are a helpful assistant.", receivedHistory[0].Content)
	assert.Equal(t, "user", receivedHistory[1].Role)
	assert.Equal(t, "Hello", receivedHistory[1].Content)
}

func TestPostChat_ModelNotFound(t *testing.T) {
	svc := mockService(nil, testOllamaConfig().Models, "system")

	_, err := svc.PostChat(context.Background(), "nonexistent-model", nil, false)
	require.Error(t, err)
}

func TestPostChat_OllamaError(t *testing.T) {
	mock := &mockOllamaClient{
		err: errors.New("connection refused"),
	}

	svc := mockService(mock, testOllamaConfig().Models, "system")
	_, err := svc.PostChat(context.Background(), "llama3.2", nil, false)
	assert.Error(t, err)
}

func TestPostChat_ForwardsThinkingFlag(t *testing.T) {
	var receivedThinking bool

	mock := &mockWithCapture{
		mockOllamaClient: &mockOllamaClient{response: ollama.MessageResponse{}},
		onCallThinking:   func(thinking bool) { receivedThinking = thinking },
	}

	svc := mockService(mock, testOllamaConfig().Models, "system")
	_, err := svc.PostChat(context.Background(), "llama3.2", nil, true)

	require.NoError(t, err)
	assert.True(t, receivedThinking)
}

func TestPostChat_InvalidProvider(t *testing.T) {
	svc := mockService(nil, []config.Model{
		{Name: "custom", Id: "custom-model", Description: "", Provider: "unknown-provider"},
	}, "system")

	_, err := svc.PostChat(context.Background(), "custom", nil, false)
	assert.EqualError(t, err, "invalid provider")
}

func TestPostChat_ToolCallsPassthrough(t *testing.T) {
	mock := &mockOllamaClient{
		response: ollama.MessageResponse{
			Response: ollama.HistoryMessage{
				Role:    "assistant",
				Content: "",
				ToolCalls: []tools.ToolCall{
					{Function: tools.Function{Name: "get_current_time", Arguments: nil}},
				},
			},
			TokenCount: 8,
		},
	}

	svc := mockService(mock, testOllamaConfig().Models, "system")
	resp, err := svc.PostChat(context.Background(), "llama3.2", nil, false)

	require.NoError(t, err)
	require.Len(t, resp.Response.ToolCalls, 1)
	assert.Equal(t, "get_current_time", resp.Response.ToolCalls[0].Function.Name)
}

func TestGenerateConversationTitle_Success(t *testing.T) {
	mock := &mockOllamaClient{
		response: ollama.MessageResponse{
			Response: ollama.HistoryMessage{
				Role:    "assistant",
				Content: "Hello World Chat",
			},
		},
	}

	svc := mockService(mock, testOllamaConfig().Models, "system")
	title, err := svc.GenerateConversationTitle(context.Background(), "hello world", "llama3.2")

	require.NoError(t, err)
	assert.Equal(t, "Hello World Chat", title)
}

func TestGenerateConversationTitle_EmptyResponse(t *testing.T) {
	mock := &mockOllamaClient{
		response: ollama.MessageResponse{
			Response: ollama.HistoryMessage{
				Role:    "assistant",
				Content: "",
			},
		},
	}

	svc := mockService(mock, testOllamaConfig().Models, "system")
	_, err := svc.GenerateConversationTitle(context.Background(), "hello world", "llama3.2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response when requested for a title")
}

func TestGenerateConversationTitle_PostChatError(t *testing.T) {
	mock := &mockOllamaClient{
		err: errors.New("ollama down"),
	}

	svc := mockService(mock, testOllamaConfig().Models, "system")
	_, err := svc.GenerateConversationTitle(context.Background(), "hello world", "llama3.2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error while generating title")
}

func TestGenerateConversationTitle_ModelNotFound(t *testing.T) {
	svc := mockService(nil, testOllamaConfig().Models, "system")
	_, err := svc.GenerateConversationTitle(context.Background(), "hello world", "no-such-model")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error while generating title")
}

func TestPostChat_OpenRouter_RoutesCorrectly(t *testing.T) {
	mock := &mockOpenRouterClient{
		response: openrouter.MessageResponse{
			Choices: []struct {
				Message openrouter.HistoryMessage `json:"message"`
			}{
				{Message: openrouter.HistoryMessage{Role: "assistant", Content: toPtrOr("OR response!")}},
			},
			Usage: struct {
				PromptTokens int `json:"prompt_tokens"`
				TotalTokens  int `json:"total_tokens"`
			}{PromptTokens: 8, TotalTokens: 20},
		},
	}

	svc := Service{
		systemPrompt:      HistoryMessage{Role: "system", Content: "sys"},
		models:            testOpenRouterConfig().Models,
		openrouterService: mock,
	}

	resp, err := svc.PostChat(context.Background(), "gpt4", []HistoryMessage{{Role: "user", Content: "Hi"}}, false)

	require.NoError(t, err)
	assert.Equal(t, "gpt4", resp.Model)
	assert.Equal(t, "OR response!", resp.Response.Content)
	assert.Equal(t, 20, resp.TokenCount)
	assert.Equal(t, 8, resp.PromptTokenCount)
}

func TestPostChat_OpenRouter_Error(t *testing.T) {
	mock := &mockOpenRouterClient{err: errors.New("openrouter unavailable")}

	svc := Service{
		systemPrompt:      HistoryMessage{Role: "system", Content: "sys"},
		models:            testOpenRouterConfig().Models,
		openrouterService: mock,
	}

	_, err := svc.PostChat(context.Background(), "gpt4", nil, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "openrouter unavailable")
}

func TestPostChat_OpenRouter_NilContentHandled(t *testing.T) {
	mock := &mockOpenRouterClient{
		response: openrouter.MessageResponse{
			Choices: []struct {
				Message openrouter.HistoryMessage `json:"message"`
			}{
				{Message: openrouter.HistoryMessage{Role: "assistant", Content: nil}},
			},
		},
	}

	svc := Service{
		systemPrompt:      HistoryMessage{Role: "system", Content: "sys"},
		models:            testOpenRouterConfig().Models,
		openrouterService: mock,
	}

	resp, err := svc.PostChat(context.Background(), "gpt4", nil, false)

	require.NoError(t, err)
	assert.Equal(t, "", resp.Response.Content)
}

func TestConvertOpenrouterHistoryToInferenceFormat_ToolCallParseError(t *testing.T) {
	svc := Service{}

	emptyStr := ""
	h := openrouter.HistoryMessage{
		Role:    "assistant",
		Content: &emptyStr,
		ToolCalls: []tools.ToolCall{
			{Function: tools.Function{Name: "test", Arguments: json.RawMessage("not a string")}},
		},
	}

	result := svc.convertOpenrouterHistoryToInferenceFormat(h)
	assert.Nil(t, result.ToolCalls)
}

func TestConvertFullHistoryToOpenrouterFormat_Basic(t *testing.T) {
	svc := Service{}

	input := []HistoryMessage{
		{Role: "system", Content: "you are helpful"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: ""},
	}

	result := svc.convertFullHistoryToOpenrouterFormat(input)

	require.Len(t, result, 3)
	assert.Equal(t, "system", result[0].Role)
	assert.Equal(t, "you are helpful", *result[0].Content)
	assert.Equal(t, "user", result[1].Role)
	assert.Equal(t, "hello", *result[1].Content)
	assert.Nil(t, result[2].Content)
}

func TestConvertFullHistoryToOpenrouterFormat_ToolCallsInAssistant(t *testing.T) {
	svc := Service{}

	toolCallJSON := `[{"function":{"name":"test","arguments":null},"type":"function","id":"call123"}]`
	input := []HistoryMessage{
		{Role: "assistant", Content: toolCallJSON, ToolCalls: nil},
	}

	result := svc.convertFullHistoryToOpenrouterFormat(input)

	require.Len(t, result, 1)
	assert.True(t, len(result[0].ToolCalls) > 0)
	assert.Equal(t, "test", result[0].ToolCalls[0].Function.Name)
	assert.Nil(t, result[0].Content)
}

func TestConvertFullHistoryToOpenrouterFormat_ToolCallIdIncluded(t *testing.T) {
	svc := Service{}

	input := []HistoryMessage{
		{Role: "user", Content: "hi", ToolCallId: "tc-123"},
	}

	result := svc.convertFullHistoryToOpenrouterFormat(input)

	assert.Equal(t, "tc-123", result[0].ToolCallId)
}

func TestPostChat_MixedProviders_RoutesCorrectly(t *testing.T) {
	ollamaMock := &mockOllamaClient{
		response: ollama.MessageResponse{
			Response: ollama.HistoryMessage{Role: "assistant", Content: "ollama reply"},
		},
	}

	openrouterMock := &mockOpenRouterClient{
		response: openrouter.MessageResponse{
			Choices: []struct {
				Message openrouter.HistoryMessage `json:"message"`
			}{
				{Message: openrouter.HistoryMessage{Role: "assistant", Content: toPtrOr("or reply")}},
			},
		},
	}

	svc := Service{
		systemPrompt: HistoryMessage{Role: "system", Content: "sys"},
		models: []config.Model{
			{Name: "local", Id: "llama3.2:latest", Provider: "ollama"},
			{Name: "cloud", Id: "openai/gpt-4", Provider: "openrouter"},
		},
		ollamaService:     ollamaMock,
		openrouterService: openrouterMock,
	}

	ollamaResp, err := svc.PostChat(context.Background(), "local", nil, false)
	require.NoError(t, err)
	assert.Equal(t, "local", ollamaResp.Model)
	assert.Equal(t, "ollama reply", ollamaResp.Response.Content)

	orResp, err := svc.PostChat(context.Background(), "cloud", nil, false)
	require.NoError(t, err)
	assert.Equal(t, "cloud", orResp.Model)
	assert.Equal(t, "or reply", orResp.Response.Content)
}

func TestConvertOpenrouterHistoryToInferenceFormat_ValidToolCallArgs(t *testing.T) {
	svc := Service{}

	emptyStr := ""
	// OpenRouter returns arguments as a JSON-encoded string, e.g. '"{\"q\":\"hi\"}"'.
	h := openrouter.HistoryMessage{
		Role:    "assistant",
		Content: &emptyStr,
		ToolCalls: []tools.ToolCall{
			{
				Function: tools.Function{
					Name:      "web_search",
					Arguments: json.RawMessage(`"{\"query\":\"hello\"}"`),
				},
			},
		},
	}

	result := svc.convertOpenrouterHistoryToInferenceFormat(h)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "web_search", result.ToolCalls[0].Function.Name)
	// After unmarshaling the JSON string and re-encoding, the args RawMessage holds
	// the inner string content (without surrounding quotes) per current behavior.
	assert.Equal(t, json.RawMessage(`{"query":"hello"}`), result.ToolCalls[0].Function.Arguments)
}

func TestConvertOpenrouterHistoryToInferenceFormat_NilContentPreserved(t *testing.T) {
	svc := Service{}

	h := openrouter.HistoryMessage{Role: "assistant", Content: nil}
	result := svc.convertOpenrouterHistoryToInferenceFormat(h)

	assert.Equal(t, "", result.Content)
	assert.Equal(t, "assistant", result.Role)
	assert.Nil(t, result.ToolCalls)
}

func TestConvertFullHistoryToOpenrouterFormat_PlainAssistantContent(t *testing.T) {
	svc := Service{}

	input := []HistoryMessage{
		{Role: "assistant", Content: "Sure, here is the answer."},
	}

	result := svc.convertFullHistoryToOpenrouterFormat(input)

	require.Len(t, result, 1)
	require.NotNil(t, result[0].Content)
	assert.Equal(t, "Sure, here is the answer.", *result[0].Content)
	assert.Nil(t, result[0].ToolCalls)
}

func TestConvertFullHistoryToOpenrouterFormat_ValidJSONNotToolCallArray(t *testing.T) {
	svc := Service{}

	input := []HistoryMessage{
		{Role: "assistant", Content: `{"some":"object","not":"a tool call array"}`},
	}

	result := svc.convertFullHistoryToOpenrouterFormat(input)

	require.Len(t, result, 1)
	require.NotNil(t, result[0].Content)
	assert.Equal(t, `{"some":"object","not":"a tool call array"}`, *result[0].Content)
	assert.Nil(t, result[0].ToolCalls)
}

type mockWithCapture struct {
	*mockOllamaClient
	onCall           func(string)
	onCallAndHistory func(string, []ollama.HistoryMessage)
	onCallThinking   func(bool)
}

func (m *mockWithCapture) PostChat(ctx context.Context, modelID string, history []ollama.HistoryMessage, thinking bool) (ollama.MessageResponse, error) {
	if m.onCall != nil {
		m.onCall(modelID)
	}
	if m.onCallAndHistory != nil {
		m.onCallAndHistory(modelID, history)
	}
	if m.onCallThinking != nil {
		m.onCallThinking(thinking)
	}
	return m.mockOllamaClient.PostChat(ctx, modelID, history, thinking)
}
