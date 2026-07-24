package conversation

import (
	"akernel/internal/config"
	"akernel/internal/handlers/dto"
	convoservice "akernel/internal/service/conversation"
	"akernel/internal/service/inference"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	models            []config.Model
	postChatResponses []inference.MessageResponse
	postChatIndex     int
	titleGenResponse  string
}

func (m *mockInferenceClient) GetModels() []config.Model {
	return m.models
}

func (m *mockInferenceClient) PostChat(_ context.Context, _ string, _ []inference.HistoryMessage, _ bool) (inference.MessageResponse, error) {
	defer func() { m.postChatIndex++ }()

	if m.postChatIndex >= len(m.postChatResponses) {
		return inference.MessageResponse{}, errors.New("mock: no more responses")
	}
	return m.postChatResponses[m.postChatIndex], nil
}

func (m *mockInferenceClient) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return m.titleGenResponse, nil
}

type errMockInferenceClient struct{}

func (m *errMockInferenceClient) GetModels() []config.Model {
	return []config.Model{{Name: "llama3.2"}}
}

func (m *errMockInferenceClient) PostChat(_ context.Context, _ string, _ []inference.HistoryMessage, _ bool) (inference.MessageResponse, error) {
	return inference.MessageResponse{}, errors.New("service connection refused")
}

func (m *errMockInferenceClient) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

// newTestMux builds a ServeMux wired to the handler so PathValue works.
func newTestMux(handler Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /conversation", handler.GetConversations)
	mux.HandleFunc("DELETE /conversation/{id}", handler.DeleteConversation)
	mux.HandleFunc("POST /conversation/{id}/summarize", handler.PostSummarize)
	return mux
}

// --- GetConversations tests ---

func TestGetConversations_SuccessWithResults(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	insertConversation(t, db, "Meaning of life")
	insertConversation(t, db, "Go programming help")

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	req := httptest.NewRequest(http.MethodGet, "/conversation", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var result []dto.Conversation
	require.NoError(t, decodeBody(rec.Body, &result))
	require.Len(t, result, 2)
}

func TestGetConversations_EmptyDatabase(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	req := httptest.NewRequest(http.MethodGet, "/conversation", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result []dto.Conversation
	require.NoError(t, decodeBody(rec.Body, &result))
	assert.Empty(t, result)
}

func TestGetConversations_ServiceError(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	_, err := db.Exec("DROP TABLE conversations")
	require.NoError(t, err)

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	req := httptest.NewRequest(http.MethodGet, "/conversation", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- DeleteConversation tests ---

func TestDeleteConversation_Success(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "delete me")

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	req := httptest.NewRequest(http.MethodDelete, "/conversation/"+strconv.Itoa(cid), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM conversations WHERE id = ?", cid))
	assert.Equal(t, 0, count)
}

func TestDeleteConversation_NonNumericID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	req := httptest.NewRequest(http.MethodDelete, "/conversation/abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteConversation_NotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	req := httptest.NewRequest(http.MethodDelete, "/conversation/9999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeleteConversation_GenericError(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	_, err := db.Exec("DROP TABLE conversations")
	require.NoError(t, err)

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	req := httptest.NewRequest(http.MethodDelete, "/conversation/1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDeleteConversation_NegativeID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	req := httptest.NewRequest(http.MethodDelete, "/conversation/-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- PostSummarize tests ---

func TestPostSummarize_HappyPath(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "summary test")
	insertMessage(t, db, "user", "hello", cid, 10)
	insertMessage(t, db, "assistant", "hi back", cid, 20)

	mock := &mockInferenceClient{
		postChatResponses: []inference.MessageResponse{{
			Response: inference.HistoryMessage{
				Role:    "assistant",
				Content: "This is the summary.",
			},
			TokenCount: 42,
		}},
	}

	handler := Handler{service: convoservice.New(db, mock)}
	mux := newTestMux(handler)

	body := `{"name":"llama3.2"}`
	req := httptest.NewRequest(http.MethodPost, "/conversation/"+strconv.Itoa(cid)+"/summarize", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var lastSummary int32
	require.NoError(t, db.Get(&lastSummary, "SELECT last_summary FROM conversations WHERE id = ?", cid))
	assert.NotZero(t, lastSummary)
}

func TestPostSummarize_NonNumericID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	body := `{"name":"llama3.2"}`
	req := httptest.NewRequest(http.MethodPost, "/conversation/abc/summarize", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostSummarize_MalformedJSON(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "malformed json test")

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/conversation/"+strconv.Itoa(cid)+"/summarize", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostSummarize_ConversationNotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	body := `{"name":"llama3.2"}`
	req := httptest.NewRequest(http.MethodPost, "/conversation/9999/summarize", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPostSummarize_NegativeID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	handler := Handler{service: convoservice.New(db, &mockInferenceClient{})}
	mux := newTestMux(handler)

	body := `{"name":"llama3.2"}`
	req := httptest.NewRequest(http.MethodPost, "/conversation/-1/summarize", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostSummarize_EmptyResponse(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "empty response test")
	insertMessage(t, db, "user", "hello", cid, 10)

	mock := &mockInferenceClient{
		postChatResponses: []inference.MessageResponse{{
			Response: inference.HistoryMessage{
				Role:    "assistant",
				Content: "",
			},
		}},
	}

	handler := Handler{service: convoservice.New(db, mock)}
	mux := newTestMux(handler)

	body := `{"name":"llama3.2"}`
	req := httptest.NewRequest(http.MethodPost, "/conversation/"+strconv.Itoa(cid)+"/summarize", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestPostSummarize_ServiceError(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "service error test")
	insertMessage(t, db, "user", "hello", cid, 10)

	handler := Handler{service: convoservice.New(db, &errMockInferenceClient{})}
	mux := newTestMux(handler)

	body := `{"name":"llama3.2"}`
	req := httptest.NewRequest(http.MethodPost, "/conversation/"+strconv.Itoa(cid)+"/summarize", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func decodeBody(r io.Reader, target interface{}) error {
	return json.NewDecoder(r).Decode(target)
}
