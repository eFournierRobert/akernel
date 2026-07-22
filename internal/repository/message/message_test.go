package message

import (
	"akernel/internal/repository"
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

	err = goose.Up(db.DB, "../migrations")
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

func insertMessage(t *testing.T, db *sqlx.DB, role, content string, conversationID, tokenCount int) int {
	t.Helper()
	ts := time.Now().Unix()
	res, err := db.Exec(
		"INSERT INTO messages (role, content, thinking, datetime, conversation_id, token_count) VALUES (?, ?, '', ?, ?, ?)",
		role, content, ts, conversationID, tokenCount,
	)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	return int(id)
}

func TestInsertMessage_Success(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "test")

	err := repo.InsertMessage(repository.Message{
		Role:           "user",
		Content:        "hello",
		Thinking:       "",
		DateTime:       time.Now(),
		ConversationId: cid,
		TokenCount:     50,
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM messages WHERE conversation_id = ?", cid))
	assert.Equal(t, 1, count)
}

func TestInsertMessage_FKViolation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	err := repo.InsertMessage(repository.Message{
		Role:           "user",
		Content:        "orphan",
		DateTime:       time.Now(),
		ConversationId: 9999,
		TokenCount:     10,
	})
	assert.Error(t, err)
}

func TestSelectMessagesOfConvo_NoSummary(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "no summary")
	insertMessage(t, db, "user", "first", cid, 10)
	insertMessage(t, db, "assistant", "reply", cid, 20)

	messages, err := repo.SelectMessagesOfConvo(cid)
	require.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "first", messages[0].Content)
	assert.Equal(t, "assistant", messages[1].Role)
	assert.Equal(t, "reply", messages[1].Content)
}

func TestSelectMessagesOfConvo_WithSummary(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "with summary")
	summaryID := insertMessage(t, db, "user", "original", cid, 30)
	insertMessage(t, db, "assistant", "detailed answer", cid, 50)
	insertMessage(t, db, "user", "follow up", cid, 20)

	_, err := db.Exec("UPDATE conversations SET last_summary = ? WHERE id = ?", summaryID, cid)
	require.NoError(t, err)

	messages, err := repo.SelectMessagesOfConvo(cid)
	require.NoError(t, err)
	// Summary prepended + messages with id > summary id
	assert.Len(t, messages, 3)
	assert.Equal(t, "original", messages[0].Content)
}

func TestSelectMessagesOfConvo_CrossConversationSummary(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid1 := insertConversation(t, db, "summary source")
	cid2 := insertConversation(t, db, "reads summary")
	sumMsgID := insertMessage(t, db, "assistant", "summarized", cid1, 40)
	insertMessage(t, db, "user", "after summary", cid2, 10)

	_, err := db.Exec("UPDATE conversations SET last_summary = ? WHERE id = ?", sumMsgID, cid2)
	require.NoError(t, err)

	messages, err := repo.SelectMessagesOfConvo(cid2)
	require.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "summarized", messages[0].Content)
}

func TestSelectMessagesOfConvo_EmptyConversation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "empty")

	messages, err := repo.SelectMessagesOfConvo(cid)
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestSelectMessagesOfConvo_NonExistentConversation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	_, err := repo.SelectMessagesOfConvo(9999)
	assert.Error(t, err)
}

func TestPopLastUserMessage_DeletesLatestUserOnly(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "pop test")
	insertMessage(t, db, "user", "first", cid, 10)
	insertMessage(t, db, "assistant", "reply", cid, 20)
	insertMessage(t, db, "user", "second", cid, 15)

	require.NoError(t, repo.PopLastUserMessage(cid))

	var remaining string
	require.NoError(t, db.Get(&remaining,
		"SELECT content FROM messages WHERE role = 'user' AND conversation_id = ?", cid))
	assert.Equal(t, "first", remaining)

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM messages WHERE conversation_id = ?", cid))
	assert.Equal(t, 2, count) // "first" user + assistant reply remain
}

func TestPopLastUserMessage_NoUserMessages(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "no users")
	insertMessage(t, db, "assistant", "only assistant", cid, 30)

	err := repo.PopLastUserMessage(cid)
	assert.NoError(t, err)
}

func TestSelectFullConversation_OrderedById(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "full")
	insertMessage(t, db, "user", "q1", cid, 10)
	insertMessage(t, db, "assistant", "a1", cid, 20)
	insertMessage(t, db, "user", "q2", cid, 5)

	messages, err := repo.SelectFullConversation(cid)
	require.NoError(t, err)
	assert.Len(t, messages, 3)
	assert.Equal(t, "q1", messages[0].Content)
	assert.Equal(t, "a1", messages[1].Content)
	assert.Equal(t, "q2", messages[2].Content)
}

func TestSelectFullConversation_Empty(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "void")

	messages, err := repo.SelectFullConversation(cid)
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestAddTokenToLatestUserMessage(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "token")
	insertMessage(t, db, "user", "older", cid, 10)
	insertMessage(t, db, "assistant", "reply", cid, 20)
	insertMessage(t, db, "user", "latest", cid, 5)

	require.NoError(t, repo.AddTokenToLatestUserMessage(cid, 999))

	var tokens int
	require.NoError(t, db.Get(&tokens,
		"SELECT token_count FROM messages WHERE content = 'latest' AND conversation_id = ?", cid))
	assert.Equal(t, 999, tokens)

	// Older user message is untouched
	var olderTokens int
	require.NoError(t, db.Get(&olderTokens,
		"SELECT token_count FROM messages WHERE content = 'older' AND conversation_id = ?", cid))
	assert.Equal(t, 10, olderTokens)
}

func TestSelectLastAssistantMessage(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "last asst")
	insertMessage(t, db, "user", "greet", cid, 10)
	insertMessage(t, db, "assistant", "first reply", cid, 20)
	insertMessage(t, db, "user", "again", cid, 5)
	insertMessage(t, db, "assistant", "second reply", cid, 30)

	msg, err := repo.SelectLastAssistantMessage(cid)
	require.NoError(t, err)
	assert.NotZero(t, msg.ID)
}

func TestSelectLastAssistantMessage_NoAssistantMessages(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "no asst")
	insertMessage(t, db, "user", "only user", cid, 10)

	_, err := repo.SelectLastAssistantMessage(cid)
	assert.Error(t, err)
}

func TestSelectAllMessagesAfterLastPrompt_Basic(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "after prompt")
	insertMessage(t, db, "user", "earlier", cid, 5)
	insertMessage(t, db, "assistant", "earlier reply", cid, 10)
	insertMessage(t, db, "user", "my question", cid, 20)
	insertMessage(t, db, "assistant", "answer", cid, 30)
	insertMessage(t, db, "user", "follow up", cid, 15)

	messages, err := repo.SelectAllMessagesAfterLastPrompt(cid, "my question")
	require.NoError(t, err)
	assert.Len(t, messages, 2)
}

func TestSelectAllMessagesAfterLastPrompt_MinIdOnDuplicate(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "repeats")
	insertMessage(t, db, "user", "first", cid, 5)
	insertMessage(t, db, "assistant", "a1", cid, 10)
	insertMessage(t, db, "user", "repeat", cid, 20)
	insertMessage(t, db, "assistant", "a2", cid, 30)
	insertMessage(t, db, "user", "repeat", cid, 15)
	insertMessage(t, db, "assistant", "a3", cid, 25)

	messages, err := repo.SelectAllMessagesAfterLastPrompt(cid, "repeat")
	require.NoError(t, err)
	// MIN(id) of "repeat" is the first occurrence. Messages after that: a2, repeat(2nd), a3
	assert.Len(t, messages, 3)
}

func TestSelectAllMessagesAfterLastPrompt_NotFound(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "no match")
	insertMessage(t, db, "user", "hello", cid, 10)
	insertMessage(t, db, "assistant", "hi", cid, 20)

	messages, err := repo.SelectAllMessagesAfterLastPrompt(cid, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, messages)
}