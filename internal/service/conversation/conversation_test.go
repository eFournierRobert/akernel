package conversation

import (
	"akernel/internal/config"
	"akernel/internal/handlers/dto"
	"akernel/internal/repository/conversation"
	"akernel/internal/repository/message"
	"akernel/internal/service"
	"akernel/internal/service/inference"
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

	err = goose.SetDialect("sqlite3")
	require.NoError(t, err)

	err = goose.Up(db.DB, "../../repository/migrations")
	require.NoError(t, err)

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
	response inference.MessageResponse
	err      error
}

func (m *mockInferenceClient) GetModels() []config.Model {
	return []config.Model{}
}

func (m *mockInferenceClient) PostChat(_ context.Context, _ string, _ []inference.HistoryMessage, _ bool) (inference.MessageResponse, error) {
	return m.response, m.err
}

func (m *mockInferenceClient) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func newTestService(t *testing.T, db *sqlx.DB, mock inference.InferenceClient) Service {
	t.Helper()
	return Service{
		repository:  conversation.New(db),
		messageRepo: message.New(db),
		inference:   mock,
	}
}

func TestFetchConversations_Populated(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid1 := insertConversation(t, db, "first")
	cid2 := insertConversation(t, db, "second")

	svc := newTestService(t, db, &mockInferenceClient{})
	result, err := svc.FetchConversations()

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, &dto.Conversation{Id: cid1, Title: "first"}, &result[0])
	assert.Equal(t, &dto.Conversation{Id: cid2, Title: "second"}, &result[1])
}

func TestFetchConversations_Empty(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	svc := newTestService(t, db, &mockInferenceClient{})
	result, err := svc.FetchConversations()

	require.NoError(t, err)
	assert.Equal(t, []dto.Conversation{}, result)
}

func TestDeleteConversation_Success(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "delete me")

	svc := newTestService(t, db, &mockInferenceClient{})
	err := svc.DeleteConversation(cid)

	assert.NoError(t, err)

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM conversations WHERE id = ?", cid))
	assert.Equal(t, 0, count)
}

func TestDeleteConversation_NotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	svc := newTestService(t, db, &mockInferenceClient{})
	err := svc.DeleteConversation(9999)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrConversationDoesNotExist))
}

func TestCreateNewSummary_HappyPath(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "summary test")
	insertMessage(t, db, "user", "hello", cid, 10)
	insertMessage(t, db, "assistant", "hi back", cid, 20)

	svc := newTestService(t, db, &mockInferenceClient{
		response: inference.MessageResponse{
			Response: inference.HistoryMessage{
				Role:    "assistant",
				Content: "This is the summary.",
			},
			TokenCount: 42,
		},
	})

	err := svc.CreateNewSummary(context.Background(), cid, "llama3.2")
	require.NoError(t, err)

	var lastSummary int32
	require.NoError(t, db.Get(&lastSummary, "SELECT last_summary FROM conversations WHERE id = ?", cid))
	assert.NotZero(t, lastSummary)

	var summaryContent string
	require.NoError(t, db.Get(&summaryContent, "SELECT content FROM messages WHERE id = ?", lastSummary))
	assert.Equal(t, "This is the summary.", summaryContent)
}

func TestCreateNewSummary_ConversationNotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	svc := newTestService(t, db, &mockInferenceClient{})
	err := svc.CreateNewSummary(context.Background(), 9999, "llama3.2")

	assert.Error(t, err)
}

func TestCreateNewSummary_EmptyOllamaResponse(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "empty response test")
	insertMessage(t, db, "user", "hello", cid, 10)

	svc := newTestService(t, db, &mockInferenceClient{
		response: inference.MessageResponse{
			Response: inference.HistoryMessage{
				Role:    "assistant",
				Content: "",
			},
		},
	})

	err := svc.CreateNewSummary(context.Background(), cid, "llama3.2")
	assert.Error(t, err)
}
