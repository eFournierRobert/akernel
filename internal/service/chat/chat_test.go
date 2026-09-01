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
	"database/sql"
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
		"INSERT INTO messages (role, content, thinking, datetime, conversation_id, token_count, tool_call_id) VALUES (?, ?, '', ?, ?, ?, NULL)",
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
		ConversationId: 0,
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
		ConversationId: 0,
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
		{"negative_sentinel", -1},
		{"negative_out_of_bounds", -2},
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

func TestGeneratePromptAnswer_ToolCallLoop_ContextCancelled(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "tool ctx cancel")

	firstResponse := inference.MessageResponse{
		Response: inference.HistoryMessage{
			Role: "assistant",
			ToolCalls: []tools.ToolCall{
				{Function: tools.Function{Name: "get_current_time", Arguments: nil}, Id: "call-abc"},
			},
		},
		TokenCount: 10,
	}

	mock := &mockInferenceClient{
		models:            []config.Model{{Name: "llama3.2"}},
		postChatResponses: []inference.MessageResponse{firstResponse},
		postChatErr:       context.Canceled,
	}

	svc := newTestService(t, db, mock)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GeneratePromptAnswer(ctx, &dto.Prompt{
		Model:          "llama3.2",
		Message:        "What time is it?",
		ConversationId: cid,
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestInsertToolAnswer_PersistsToolCallId(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "toolcall persistence")

	mockCall := []tools.ToolCall{
		{Function: tools.Function{Name: "get_current_time", Arguments: nil}, Id: "call-xyz"},
	}

	mock := &mockInferenceClient{
		models: []config.Model{{Name: "llama3.2"}},
		postChatResponses: []inference.MessageResponse{
			{
				Response: inference.HistoryMessage{
					Role:      "assistant",
					Content:   "",
					ToolCalls: mockCall,
				},
				TokenCount: 10,
			},
			{
				Response: inference.HistoryMessage{
					Role:    "assistant",
					Content: "Done.",
				},
				TokenCount:       20,
				PromptTokenCount: 5,
			},
		},
	}

	svc := newTestService(t, db, mock)
	_, err := svc.GeneratePromptAnswer(context.Background(), &dto.Prompt{
		Model:          "llama3.2",
		Message:        "What time is it?",
		ConversationId: cid,
	})

	require.NoError(t, err)

	var toolCallId sql.NullString
	require.NoError(t, db.Get(&toolCallId,
		"SELECT tool_call_id FROM messages WHERE role = 'tool' AND conversation_id = ? ORDER BY id DESC LIMIT 1", cid))
	assert.True(t, toolCallId.Valid)
	assert.Equal(t, "call-xyz", toolCallId.String)
}

func TestFetchConversationHistory_IncludesToolCallId(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "toolcall id history")

	_, err := db.Exec(
		"INSERT INTO messages (role, content, thinking, datetime, conversation_id, token_count, tool_call_id) VALUES (?, ?, '', ?, ?, 0, ?)",
		"tool", "result", time.Now().Unix(), cid, "call-1",
	)
	require.NoError(t, err)

	insertMessage(t, db, "user", "question", cid, 5)

	svc := &Service{
		messageRepo: message.New(db),
		convoRepo:   conversation.New(db),
		inference:   &mockInferenceClient{models: []config.Model{{Name: "llama3.2"}}},
	}

	history, err := svc.fetchConversationHistory(cid)
	require.NoError(t, err)
	require.Len(t, history, 2)

	// First message is the tool row (lower id). Confirm ToolCallId round-tripped.
	assert.Equal(t, "tool", history[0].Role)
	assert.Equal(t, "call-1", history[0].ToolCallId)
	// Plain user message has no tool_call_id → empty string.
	assert.Equal(t, "user", history[1].Role)
	assert.Empty(t, history[1].ToolCallId)
}

// cancelOnSecondCallMock is a mockInferenceClient variant that invokes a stored cancel
// function on its second PostChat call, so the tool-call loop sees a cancelled context
// while PostChat itself returns no error (isolating the nil-err / ctx-cancelled branch).
type cancelOnSecondCallMock struct {
	models            []config.Model
	postChatResponses []inference.MessageResponse
	index             int
	cancel            func()
}

func (m *cancelOnSecondCallMock) GetModels() []config.Model { return m.models }

func (m *cancelOnSecondCallMock) PostChat(_ context.Context, _ string, _ []inference.HistoryMessage, _ bool) (inference.MessageResponse, error) {
	defer func() { m.index++ }()
	if m.index == 1 {
		m.cancel()
	}
	if m.index < len(m.postChatResponses) {
		return m.postChatResponses[m.index], nil
	}
	return inference.MessageResponse{}, errors.New("mock: no more responses")
}

func (m *cancelOnSecondCallMock) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return "title", nil
}

// TestGeneratePromptAnswer_ToolCallLoop_NilErrContextCancelled isolates the branch at
// chat.go ~line 82 where, inside the tool-call loop, getHistoryAndPostChat returns a
// nil error but ctx is cancelled. The first iteration completes a tool call; the mock
// cancels ctx just before the second PostChat, so the loop must bail with context.Canceled.
func TestGeneratePromptAnswer_ToolCallLoop_NilErrContextCancelled(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "nil err ctx cancel")

	firstResponse := inference.MessageResponse{
		Response: inference.HistoryMessage{
			Role: "assistant",
			ToolCalls: []tools.ToolCall{
				{Function: tools.Function{Name: "get_current_time", Arguments: nil}, Id: "call-abc"},
			},
		},
		TokenCount: 10,
	}
	secondResponse := inference.MessageResponse{
		Response: inference.HistoryMessage{
			Role:    "assistant",
			Content: "Done.",
		},
		TokenCount:       20,
		PromptTokenCount: 5,
	}

	ctx, cancel := context.WithCancel(context.Background())
	mock := &cancelOnSecondCallMock{
		models:            []config.Model{{Name: "llama3.2"}},
		postChatResponses: []inference.MessageResponse{firstResponse, secondResponse},
		cancel:            cancel,
	}

	svc := newTestService(t, db, mock)
	_, err := svc.GeneratePromptAnswer(ctx, &dto.Prompt{
		Model:          "llama3.2",
		Message:        "What time is it?",
		ConversationId: cid,
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
