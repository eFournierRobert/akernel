package chat

import (
	"akernel/internal/handlers/dto"
	"akernel/internal/service"
	"akernel/internal/service/ollama"
	"akernel/internal/service/ollama/tools"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", "file::memory:?cache=shared")
	require.NoError(t, err)

	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.Up(db.DB, "../../repository/migrations"))

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	return db
}

func insertConversation(t *testing.T, db *sqlx.DB, title string) int {
	t.Helper()
	res, err := db.Exec("INSERT INTO conversations (title) VALUES (?)", title)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	return int(id)
}

func insertMessage(t *testing.T, db *sqlx.DB, role, content string, conversationID, tokenCount int) {
	t.Helper()
	ts := time.Now().Unix()
	_, err := db.Exec(
		"INSERT INTO messages (role, content, thinking, datetime, conversation_id, token_count) VALUES (?, ?, '', ?, ?, ?)",
		role, content, ts, conversationID, tokenCount,
	)
	require.NoError(t, err)
}

type mockOllamaClient struct {
	models             []ollama.Model
	postChatResponses  []ollama.MessageResponse
	postChatIndex      int
	postChatErr        error
	titleGenResponse   string
	titleGenErr        error
	modelId            string
}

func (m *mockOllamaClient) GetModels() []ollama.Model {
	return m.models
}

func (m *mockOllamaClient) PostChat(_ context.Context, _ string, _ []ollama.HistoryMessage, _ bool) (ollama.MessageResponse, error) {
	defer func() { m.postChatIndex++ }()

	if m.postChatErr != nil {
		return ollama.MessageResponse{}, m.postChatErr
	}

	if m.postChatIndex >= len(m.postChatResponses) {
		return ollama.MessageResponse{}, errors.New("mock: no more responses")
	}

	return m.postChatResponses[m.postChatIndex], nil
}

func (m *mockOllamaClient) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return m.titleGenResponse, m.titleGenErr
}

func (m *mockOllamaClient) GetModelId(_ string) string {
	return m.modelId
}

func TestGeneratePromptAnswer_HappyPath_NewConversation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	mock := &mockOllamaClient{
		models: []ollama.Model{{Name: "llama3.2"}},
		modelId: "llama3.2:latest",
		titleGenResponse: "New Chat",
		postChatResponses: []ollama.MessageResponse{{
			Response: ollama.HistoryMessage{
				Role:    "assistant",
				Content: "Hello there!",
			},
			TokenCount:       10,
			PromptTokenCount: 5,
		}},
	}

	svc := NewChatService(db, mock)
	result, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
		Model:          "llama3.2",
		Message:        "Hey",
		ConversationId: -1,
	})

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Hello there!", result[0].Message)
}

func TestGeneratePromptAnswer_HappyPath_ExistingConversation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "existing")

	mock := &mockOllamaClient{
		models: []ollama.Model{{Name: "llama3.2"}},
		modelId: "llama3.2:latest",
		postChatResponses: []ollama.MessageResponse{{
			Response: ollama.HistoryMessage{
				Role:    "assistant",
				Content: "Greeting from existing convo!",
			},
			TokenCount:       10,
			PromptTokenCount: 5,
		}},
	}

	svc := NewChatService(db, mock)
	result, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
		Model:          "llama3.2",
		Message:        "Hello",
		ConversationId: cid,
	})

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Greeting from existing convo!", result[0].Message)
}

func TestGeneratePromptAnswer_WithToolCalls_Success(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "tool calls")

	mockCall := []tools.ToolCall{
		{Function: tools.Function{Name: "get_current_time", Arguments: nil}},
	}
	firstResponse := ollama.MessageResponse{
		Response: ollama.HistoryMessage{
			Role:      "assistant",
			Content:   "",
			ToolCalls: mockCall,
		},
		TokenCount: 10,
	}
	secondResponse := ollama.MessageResponse{
		Response: ollama.HistoryMessage{
			Role:    "assistant",
			Content: "The time was returned by the tool.",
		},
		TokenCount:       20,
		PromptTokenCount: 15,
	}

	mock := &mockOllamaClient{
		models: []ollama.Model{{Name: "llama3.2"}},
		modelId: "llama3.2:latest",
		postChatResponses: []ollama.MessageResponse{firstResponse, secondResponse},
	}

	svc := NewChatService(db, mock)
	result, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
		Model:          "llama3.2",
		Message:        "What time is it?",
		ConversationId: cid,
	})

	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestGeneratePromptAnswer_ModelNotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	mock := &mockOllamaClient{
		models: []ollama.Model{{Name: "llama3.2"}},
	}

	svc := NewChatService(db, mock)
	_, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
		Model:          "nonexistent-model",
		Message:        "Hello",
		ConversationId: -1,
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrModelNotFound))
}

func TestGeneratePromptAnswer_InvalidConversationId(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid1 := insertConversation(t, db, "valid")
	_ = cid1

	testCases := []struct {
		name         string
		conversation int
	}{
		{"negative_out_of_bounds", -2},
		{"zero_id", 0},
		{"nonexistent_positive", 99999},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockOllamaClient{
				models: []ollama.Model{{Name: "llama3.2"}},
			}

			svc := NewChatService(db, mock)
			_, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
				Model:          "llama3.2",
				Message:        "Hello",
				ConversationId: tc.conversation,
			})

			assert.Error(t, err)
			assert.True(t, errors.Is(err, service.ErrInvalidConversation))
		})
	}
}

func TestGeneratePromptAnswer_OllamaPostChatError_RollsBack(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "rollback test")

	mock := &mockOllamaClient{
		models:  []ollama.Model{{Name: "llama3.2"}},
		modelId: "llama3.2:latest",
		postChatErr: errors.New("ollama down"),
	}

	svc := NewChatService(db, mock)
	_, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
		Model:          "llama3.2",
		Message:        "Hello",
		ConversationId: cid,
	})

	assert.Error(t, err)

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM messages WHERE conversation_id = ?", cid))
	assert.Equal(t, 0, count)
}

func TestGeneratePromptAnswer_ContextCancelled_RollsBack(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "ctx test")

	svc := NewChatService(db, &mockOllamaClient{
		models:  []ollama.Model{{Name: "llama3.2"}},
		modelId: "llama3.2:latest",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GeneratePromptAnswer(ctx, &dto.Prompt{
		Model:          "llama3.2",
		Message:        "Hello",
		ConversationId: cid,
	})

	assert.Error(t, err)

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM messages WHERE conversation_id = ?", cid))
	assert.Equal(t, 0, count)
}

func TestGeneratePromptAnswer_EmptyResponse(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "empty")

	mock := &mockOllamaClient{
		models:  []ollama.Model{{Name: "llama3.2"}},
		modelId: "llama3.2:latest",
		postChatResponses: []ollama.MessageResponse{{
			Response: ollama.HistoryMessage{
				Role:    "assistant",
				Content: "",
			},
			TokenCount: 0,
		}},
	}

	svc := NewChatService(db, mock)
	result, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
		Model:          "llama3.2",
		Message:        "Hello",
		ConversationId: cid,
	})

	assert.Error(t, err)
	assert.Nil(t, result)

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM messages WHERE conversation_id = ?", cid))
	assert.Equal(t, 0, count)
}

func TestGetConversation_Success_WithMessages(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "messages convo")
	insertMessage(t, db, "user", "hey", cid, 5)
	insertMessage(t, db, "assistant", "hi there!", cid, 10)

	mock := &mockOllamaClient{}
	svc := NewChatService(db, mock)
	result, err := svc.GetConversation(cid)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "user", result[0].Role)
	assert.Equal(t, "hey", result[0].Content)
	assert.Equal(t, "assistant", result[1].Role)
	assert.Equal(t, "hi there!", result[1].Content)
}

func TestGetConversation_Success_EmptyConversation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "empty convo")

	mock := &mockOllamaClient{}
	svc := NewChatService(db, mock)
	result, err := svc.GetConversation(cid)

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetConversation_ConversationNotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	mock := &mockOllamaClient{}
	svc := NewChatService(db, mock)
	_, err := svc.GetConversation(99999)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrConversationDoesNotExist))
}
