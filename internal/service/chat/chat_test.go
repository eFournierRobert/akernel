package chat

import (
	"akernel/internal/config"
	"akernel/internal/handlers/dto"
	"akernel/internal/repository/conversation"
	"akernel/internal/repository/message"
	"akernel/internal/service"
	"akernel/internal/service/inference"
	"akernel/internal/service/inference/tools"
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

type mockInferenceClient struct {
	models            []config.Model
	postChatResponses []inference.MessageResponse
	postChatIndex     int
	postChatErr       error
	titleGenResponse  string
	titleGenErr       error
}

func (m *mockInferenceClient) GetModels() []config.Model {
	return m.models
}

func (m *mockInferenceClient) PostChat(_ context.Context, _ string, _ []inference.HistoryMessage, _ bool) (inference.MessageResponse, error) {
	defer func() { m.postChatIndex++ }()

	if m.postChatIndex < len(m.postChatResponses) {
		return m.postChatResponses[m.postChatIndex], nil
	}

	if m.postChatErr != nil {
		return inference.MessageResponse{}, m.postChatErr
	}

	return inference.MessageResponse{}, errors.New("mock: no more responses")
}

func (m *mockInferenceClient) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return m.titleGenResponse, m.titleGenErr
}

func newTestService(t *testing.T, db *sqlx.DB, mock inference.InferenceClient) *Service {
	t.Helper()
	return &Service{
		messageRepo: message.New(db),
		convoRepo:   conversation.New(db),
		inference:   mock,
	}
}

func TestGeneratePromptAnswer_HappyPath_NewConversation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	mock := &mockInferenceClient{
		models:           []config.Model{{Name: "llama3.2"}},
		titleGenResponse: "New Chat",
		postChatResponses: []inference.MessageResponse{{
			Response: inference.HistoryMessage{
				Role:    "assistant",
				Content: "Hello there!",
			},
			TokenCount:       10,
			PromptTokenCount: 5,
		}},
	}

	svc := newTestService(t, db, mock)
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

	mock := &mockInferenceClient{
		models: []config.Model{{Name: "llama3.2"}},
		postChatResponses: []inference.MessageResponse{{
			Response: inference.HistoryMessage{
				Role:    "assistant",
				Content: "Greeting from existing convo!",
			},
			TokenCount:       10,
			PromptTokenCount: 5,
		}},
	}

	svc := newTestService(t, db, mock)
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
	firstResponse := inference.MessageResponse{
		Response: inference.HistoryMessage{
			Role:      "assistant",
			Content:   "",
			ToolCalls: mockCall,
		},
		TokenCount: 10,
	}
	secondResponse := inference.MessageResponse{
		Response: inference.HistoryMessage{
			Role:    "assistant",
			Content: "The time was returned by the tool.",
		},
		TokenCount:       20,
		PromptTokenCount: 15,
	}

	mock := &mockInferenceClient{
		models:            []config.Model{{Name: "llama3.2"}},
		postChatResponses: []inference.MessageResponse{firstResponse, secondResponse},
	}

	svc := newTestService(t, db, mock)
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

	mock := &mockInferenceClient{
		models: []config.Model{{Name: "llama3.2"}},
	}

	svc := newTestService(t, db, mock)
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

	insertConversation(t, db, "valid")

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
			mock := &mockInferenceClient{
				models: []config.Model{{Name: "llama3.2"}},
			}

			svc := newTestService(t, db, mock)
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

	mock := &mockInferenceClient{
		models:      []config.Model{{Name: "llama3.2"}},
		postChatErr: errors.New("ollama down"),
	}

	svc := newTestService(t, db, mock)
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

// TestGeneratePromptAnswer_PostChatError_PreservesPriorHistory is a bug-exposure
// test. Expected behavior: when inference fails, only the new prompt is rolled
// back. Currently the last user message is popped twice (once in
// getHistoryAndPostChat and once in GeneratePromptAnswer), silently deleting a
// prior user message.
func TestGeneratePromptAnswer_PostChatError_PreservesPriorHistory(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "rollback with history")
	insertMessage(t, db, "user", "earlier question", cid, 10)
	insertMessage(t, db, "assistant", "earlier answer", cid, 20)

	mock := &mockInferenceClient{
		models:      []config.Model{{Name: "llama3.2"}},
		postChatErr: errors.New("ollama down"),
	}

	svc := newTestService(t, db, mock)
	_, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
		Model:          "llama3.2",
		Message:        "Hello",
		ConversationId: cid,
	})

	assert.Error(t, err)

	var contents []string
	require.NoError(t, db.Select(&contents,
		"SELECT content FROM messages WHERE conversation_id = ? ORDER BY id", cid))
	assert.Equal(t, []string{"earlier question", "earlier answer"}, contents)
}

// TestGeneratePromptAnswer_ToolCallThenError_PreservesPriorHistory is a
// bug-exposure test covering the same double-pop on the error path after tool
// calls have been made.
func TestGeneratePromptAnswer_ToolCallThenError_PreservesPriorHistory(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "tool rollback with history")
	insertMessage(t, db, "user", "earlier question", cid, 10)
	insertMessage(t, db, "assistant", "earlier answer", cid, 20)

	mock := &mockInferenceClient{
		models: []config.Model{{Name: "llama3.2"}},
		postChatResponses: []inference.MessageResponse{{
			Response: inference.HistoryMessage{
				Role: "assistant",
				ToolCalls: []tools.ToolCall{
					{Function: tools.Function{Name: "get_current_time", Arguments: nil}},
				},
			},
			TokenCount: 10,
		}},
		postChatErr: errors.New("ollama down"),
	}

	svc := newTestService(t, db, mock)
	_, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
		Model:          "llama3.2",
		Message:        "What time is it?",
		ConversationId: cid,
	})

	assert.Error(t, err)

	var count int
	require.NoError(t, db.Get(&count,
		"SELECT COUNT(*) FROM messages WHERE conversation_id = ? AND content = 'earlier question'", cid))
	assert.Equal(t, 1, count)
}

// TestGeneratePromptAnswer_ContextCancelled_RollsBack is a bug-exposure test.
// Expected behavior: when the context is cancelled, GeneratePromptAnswer returns
// an error and rolls back ONLY the new prompt, leaving prior history intact.
// Currently the mock's valid response makes PostChat succeed, so the service hits
// the `ctx.Err() != nil` branch, pops a SECOND user message, and returns (nil, nil).
func TestGeneratePromptAnswer_ContextCancelled_RollsBack(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "ctx test")
	insertMessage(t, db, "user", "earlier question", cid, 10)
	insertMessage(t, db, "assistant", "earlier answer", cid, 20)

	svc := newTestService(t, db, &mockInferenceClient{
		models: []config.Model{{Name: "llama3.2"}},
		postChatResponses: []inference.MessageResponse{{
			Response: inference.HistoryMessage{
				Role:    "assistant",
				Content: "late answer",
			},
			TokenCount: 10,
		}},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GeneratePromptAnswer(ctx, &dto.Prompt{
		Model:          "llama3.2",
		Message:        "Hello",
		ConversationId: cid,
	})

	assert.Error(t, err)

	var contents []string
	require.NoError(t, db.Select(&contents,
		"SELECT content FROM messages WHERE conversation_id = ? ORDER BY id", cid))
	assert.Equal(t, []string{"earlier question", "earlier answer"}, contents)
}

func TestGeneratePromptAnswer_EmptyResponse(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "empty")

	mock := &mockInferenceClient{
		models: []config.Model{{Name: "llama3.2"}},
		postChatResponses: []inference.MessageResponse{{
			Response: inference.HistoryMessage{
				Role:    "assistant",
				Content: "",
			},
			TokenCount: 0,
		}},
	}

	svc := newTestService(t, db, mock)
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

	svc := newTestService(t, db, &mockInferenceClient{})
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

	svc := newTestService(t, db, &mockInferenceClient{})
	result, err := svc.GetConversation(cid)

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetConversation_ConversationNotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	svc := newTestService(t, db, &mockInferenceClient{})
	_, err := svc.GetConversation(99999)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrConversationDoesNotExist))
}
