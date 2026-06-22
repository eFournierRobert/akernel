package conversation

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

	err = goose.Up(db.DB, "../../../migrations")
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

func TestSelectConversations_Empty(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	conversations, err := repo.SelectConversations()
	require.NoError(t, err)
	assert.Empty(t, conversations)
}

func TestSelectConversations_Populated(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	id1 := insertConversation(t, db, "first")
	id2 := insertConversation(t, db, "second")

	conversations, err := repo.SelectConversations()
	require.NoError(t, err)
	assert.Len(t, conversations, 2)
	assert.Equal(t, id1, conversations[0].ID)
	assert.Equal(t, "first", conversations[0].Title)
	assert.Equal(t, id2, conversations[1].ID)
	assert.Equal(t, "second", conversations[1].Title)
}

func TestEnsureConversation_CreatesNew(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	convo := &repository.Conversation{Title: "new convo"}
	require.NoError(t, repo.EnsureConversation(convo))
	assert.NotZero(t, convo.ID)
	assert.Equal(t, "new convo", convo.Title)
}

func TestEnsureConversation_Idempotent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	convo1 := &repository.Conversation{Title: "duplicate"}
	require.NoError(t, repo.EnsureConversation(convo1))
	id := convo1.ID

	convo2 := &repository.Conversation{Title: "duplicate"}
	require.NoError(t, repo.EnsureConversation(convo2))
	assert.Equal(t, id, convo2.ID)
}

func TestDoesConversationExist_Existing(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "exists")
	assert.True(t, repo.DoesConversationExist(cid))
}

func TestDoesConversationExist_NonExistent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	assert.False(t, repo.DoesConversationExist(9999))
}

func TestDoesConversationExist_ZeroId(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	assert.False(t, repo.DoesConversationExist(0))
}

func TestCascadeDeleteConversation_WithChildren(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "parent")
	insertMessage(t, db, "user", "msg1", cid, 10)
	insertMessage(t, db, "assistant", "msg2", cid, 20)

	affected, err := repo.CascadeDeleteConversation(cid)
	require.NoError(t, err)
	assert.Equal(t, 1, affected)

	var msgCount int
	require.NoError(t, db.Get(&msgCount, "SELECT COUNT(*) FROM messages WHERE conversation_id = ?", cid))
	assert.Equal(t, 0, msgCount)
}

func TestCascadeDeleteConversation_NoChildren(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "no children")

	affected, err := repo.CascadeDeleteConversation(cid)
	require.NoError(t, err)
	assert.Equal(t, 1, affected)
}

func TestCascadeDeleteConversation_NonExistent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	affected, err := repo.CascadeDeleteConversation(9999)
	require.NoError(t, err)
	assert.Equal(t, 0, affected)
}

func TestSetNewSummary_Existing(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	cid := insertConversation(t, db, "summary convo")
	insertMessage(t, db, "assistant", "summary content", cid, 50)

	var msgID int
	require.NoError(t, db.Get(&msgID, "SELECT id FROM messages WHERE conversation_id = ?", cid))

	require.NoError(t, repo.SetNewSummary(cid, msgID))

	var savedSummary int32
	require.NoError(t, db.Get(&savedSummary, "SELECT last_summary FROM conversations WHERE id = ?", cid))
	assert.Equal(t, int32(msgID), savedSummary)
}

func TestSetNewSummary_NonExistent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	repo := New(db)

	err := repo.SetNewSummary(9999, 1)
	assert.NoError(t, err)
}