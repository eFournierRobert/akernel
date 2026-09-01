package chat

import (
	"akernel/internal/config"
	"akernel/internal/handlers/dto"
	chatService "akernel/internal/service/chat"
	"akernel/internal/service/inference"
	"akernel/internal/service/inference/tools"
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
	mux.HandleFunc("POST /chat", handler.Chat)
	mux.HandleFunc("GET /conversation/{id}", handler.GetFullConversation)
	return mux
}

func TestChat_HappyPath_NewConversation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	mock := &mockInferenceClient{
		models:           []config.Model{{Name: "llama3.2"}},
		titleGenResponse: "My New Chat",
		postChatResponses: []inference.MessageResponse{{
			Response: inference.HistoryMessage{
				Role:    "assistant",
				Content: "Hello!",
			},
			TokenCount:       10,
			PromptTokenCount: 5,
		}},
	}

	handler := Handler{service: chatService.NewChatService(db, mock)}
	mux := newTestMux(handler)

	body := `{"model":"llama3.2","message":"Hey","conversationId":0}`
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

	h := Handler{service: chatService.NewChatService(db, &mockInferenceClient{})}
	mux := newTestMux(h)

	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{bad json`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestChat_ModelNotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	mock := &mockInferenceClient{
		models: []config.Model{{Name: "llama3.2"}},
	}

	handler := Handler{service: chatService.NewChatService(db, mock)}
	mux := newTestMux(handler)

	body := `{"model":"nonexistent","message":"hello","conversationId":0}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestChat_InvalidConversationID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	mock := &mockInferenceClient{
		models: []config.Model{{Name: "llama3.2"}},
	}

	handler := Handler{service: chatService.NewChatService(db, mock)}
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

	handler := Handler{service: chatService.NewChatService(db, &errMockInferenceClient{})}
	mux := newTestMux(handler)

	body := `{"model":"llama3.2","message":"hello","conversationId":` + strconv.Itoa(cid) + `}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestChat_WithToolCalls_Success(t *testing.T) {
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

	handler := Handler{service: chatService.NewChatService(db, mock)}
	mux := newTestMux(handler)

	body := `{"model":"llama3.2","message":"What time is it?","conversationId":` + strconv.Itoa(cid) + `}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result []dto.Answer
	require.NoError(t, decodeBody(rec.Body, &result))
	assert.GreaterOrEqual(t, len(result), 2)
}

func TestGetFullConversation_SuccessWithMessages(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	cid := insertConversation(t, db, "messages convo")
	insertMessage(t, db, "user", "hey", cid, 5)
	insertMessage(t, db, "assistant", "hi there!", cid, 10)

	h := Handler{service: chatService.NewChatService(db, &mockInferenceClient{})}
	mux := newTestMux(h)

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

	h := Handler{service: chatService.NewChatService(db, &mockInferenceClient{})}
	mux := newTestMux(h)

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

	h := Handler{service: chatService.NewChatService(db, &mockInferenceClient{})}
	mux := newTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetFullConversation_NegativeID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	h := Handler{service: chatService.NewChatService(db, &mockInferenceClient{})}
	mux := newTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetFullConversation_ZeroID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	h := Handler{service: chatService.NewChatService(db, &mockInferenceClient{})}
	mux := newTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/0", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetFullConversation_NotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	h := Handler{service: chatService.NewChatService(db, &mockInferenceClient{})}
	mux := newTestMux(h)

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

	h := Handler{service: chatService.NewChatService(db, &mockInferenceClient{})}
	mux := newTestMux(h)

	req := httptest.NewRequest(http.MethodGet, "/conversation/"+strconv.Itoa(cid), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func decodeBody(r io.Reader, target interface{}) error {
	return json.NewDecoder(r).Decode(target)
}
