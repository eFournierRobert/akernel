package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"akernel/internal/handlers/dto"
	chatService "akernel/internal/service/chat"
	"akernel/internal/service/ollama"

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

type mockOllamaClient struct {
	models            []ollama.Model
	postChatResponses []ollama.MessageResponse
	postChatIndex     int
	titleGenResponse  string
	modelId           string
}

func (m *mockOllamaClient) GetModels() []ollama.Model {
	return m.models
}

func (m *mockOllamaClient) PostChat(_ context.Context, _ string, _ []ollama.HistoryMessage, _ bool) (ollama.MessageResponse, error) {
	defer func() { m.postChatIndex++ }()

	if m.postChatIndex >= len(m.postChatResponses) {
		return ollama.MessageResponse{}, errors.New("mock: no more responses")
	}
	return m.postChatResponses[m.postChatIndex], nil
}

func (m *mockOllamaClient) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return m.titleGenResponse, nil
}

func (m *mockOllamaClient) GetModelId(_ string) string {
	return m.modelId
}

type errOllamaClient struct{}

func (m *errOllamaClient) GetModels() []ollama.Model {
	return []ollama.Model{{Name: "llama3.2"}}
}

func (m *errOllamaClient) PostChat(_ context.Context, _ string, _ []ollama.HistoryMessage, _ bool) (ollama.MessageResponse, error) {
	return ollama.MessageResponse{}, errors.New("ollama connection refused")
}

func (m *errOllamaClient) GenerateConversationTitle(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func (m *errOllamaClient) GetModelId(_ string) string {
	return "llama3.2:latest"
}

func newTestMux(handler *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /chat", handler.Chat)
	mux.HandleFunc("GET /conversation/{id}", handler.GetFullConversation)
	return mux
}

// --- POST /chat tests ---

func TestChat_HappyPath_NewConversation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	mock := &mockOllamaClient{
		models: []ollama.Model{{Name: "llama3.2"}},
		modelId: "llama3.2:latest",
		titleGenResponse: "My New Chat",
		postChatResponses: []ollama.MessageResponse{{
			Response: ollama.HistoryMessage{
				Role:    "assistant",
				Content: "Hello!",
			},
			TokenCount:       10,
			PromptTokenCount: 5,
		}},
	}

	handler := New(mock, db)
	mux := newTestMux(handler)

	body := `{"model":"llama3.2","message":"Hey","conversationId":-1}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var result []dto.Answer
	require.NoError(t, decodeBody(rec.Body, &result))
	require.Len(t, result, 1)
	assert.Equal(t, "Hello!", result[0].Message)
}

func TestChat_MalformedJSON(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	h := Handler{service: chatService.NewChatService(db, &mockOllamaClient{})}
	mux := newTestMux(&h)

	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{bad json`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestChat_ModelNotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	mock := &mockOllamaClient{
		models: []ollama.Model{{Name: "llama3.2"}},
	}

	handler := New(mock, db)
	mux := newTestMux(handler)

	body := `{"model":"nonexistent","message":"hello","conversationId":-1}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestChat_InvalidConversationID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "valid")
	_ = cid

	mock := &mockOllamaClient{
		models: []ollama.Model{{Name: "llama3.2"}},
	}

	handler := New(mock, db)
	mux := newTestMux(handler)

	body := `{"model":"llama3.2","message":"hello","conversationId":99999}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestChat_GenericServiceError(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "error test")

	mock := &errOllamaClient{}

	handler := New(mock, db)
	mux := newTestMux(handler)

	body := `{"model":"llama3.2","message":"hello","conversationId":` + strconv.Itoa(cid) + `}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- GET /conversation/{id} tests ---

func TestGetFullConversation_SuccessWithMessages(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "messages convo")
	insertMessage(t, db, "user", "hey", cid, 5)
	insertMessage(t, db, "assistant", "hi there!", cid, 10)

	h := Handler{service: chatService.NewChatService(db, &mockOllamaClient{})}
	mux := newTestMux(&h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/"+strconv.Itoa(cid), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var result []dto.Message
	require.NoError(t, decodeBody(rec.Body, &result))
	require.Len(t, result, 2)
	assert.Equal(t, "user", result[0].Role)
	assert.Equal(t, "hey", result[0].Content)
	assert.Equal(t, "assistant", result[1].Role)
	assert.Equal(t, "hi there!", result[1].Content)
}

func TestGetFullConversation_Success_EmptyConversation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "empty convo")

	h := Handler{service: chatService.NewChatService(db, &mockOllamaClient{})}
	mux := newTestMux(&h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/"+strconv.Itoa(cid), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result []dto.Message
	require.NoError(t, decodeBody(rec.Body, &result))
	assert.Empty(t, result)
}

func TestGetFullConversation_NonNumericID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	h := Handler{service: chatService.NewChatService(db, &mockOllamaClient{})}
	mux := newTestMux(&h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetFullConversation_NegativeID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	h := Handler{service: chatService.NewChatService(db, &mockOllamaClient{})}
	mux := newTestMux(&h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetFullConversation_ZeroID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	h := Handler{service: chatService.NewChatService(db, &mockOllamaClient{})}
	mux := newTestMux(&h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/0", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetFullConversation_NotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	h := Handler{service: chatService.NewChatService(db, &mockOllamaClient{})}
	mux := newTestMux(&h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/99999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetFullConversation_GenericError(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "error test")

	_, err := db.Exec("DROP TABLE messages")
	require.NoError(t, err)

	h := Handler{service: chatService.NewChatService(db, &mockOllamaClient{})}
	mux := newTestMux(&h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/"+strconv.Itoa(cid), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func decodeBody(r interface{ String() string }, target interface{}) error {
	decoder := json.NewDecoder(strings.NewReader(r.String()))
	return decoder.Decode(target)
}
