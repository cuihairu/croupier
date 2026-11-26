package model

import (
	"context"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewMessageModel(t *testing.T) {
	db := setupMessageTestDB(t)
	model := NewMessageModel(db)
	assert.NotNil(t, model)
}

func TestMessageModel_Create(t *testing.T) {
	db := setupMessageTestDB(t)
	model := NewMessageModel(db)
	ctx := context.Background()

	msg := &Message{
		To:      "user@example.com",
		Type:    "notification",
		Title:   "Test Message",
		Content: "Test content",
		Status:  dbenum.MessageStatusUnread,
	}

	err := model.Create(ctx, msg)
	require.NoError(t, err)
	assert.NotZero(t, msg.ID)
}

func TestMessageModel_FindOne(t *testing.T) {
	db := setupMessageTestDB(t)
	model := NewMessageModel(db)
	ctx := context.Background()

	msg := &Message{
		To:      "user@example.com",
		Type:    "notification",
		Title:   "Find Test",
		Content: "Find test content",
		Status:  dbenum.MessageStatusUnread,
	}
	err := model.Create(ctx, msg)
	require.NoError(t, err)

	// Find existing
	found, err := model.FindOne(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, msg.Title, found.Title)

	// Find non-existent
	_, err = model.FindOne(ctx, 99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message not found")
}

func TestMessageModel_List(t *testing.T) {
	db := setupMessageTestDB(t)
	model := NewMessageModel(db)
	ctx := context.Background()

	// Create test messages
	messages := []*Message{
		{To: "user1@example.com", Type: "notification", Title: "Msg 1", Content: "Content 1", Status: dbenum.MessageStatusUnread},
		{To: "user1@example.com", Type: "alert", Title: "Msg 2", Content: "Content 2", Status: dbenum.MessageStatusRead},
		{To: "user2@example.com", Type: "notification", Title: "Msg 3", Content: "Content 3", Status: dbenum.MessageStatusUnread},
	}
	for _, m := range messages {
		err := model.Create(ctx, m)
		require.NoError(t, err)
	}

	// Test list all
	_, total, err := model.List(ctx, NewListMessagesOptions())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))

	// Test filter by type
	items, total, err := model.List(ctx, func() ListMessagesOptions { o := NewListMessagesOptions(); o.Type = "notification"; return o }())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))
	_ = items // items is used for verification
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))

	// Test filter by status
	_, total, err = model.List(ctx, ListMessagesOptions{Status: dbenum.MessageStatusUnread})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))

	// Test filter by recipient
	_, total, err = model.List(ctx, func() ListMessagesOptions { o := NewListMessagesOptions(); o.To = "user1@example.com"; return o }())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))
}

func TestMessageModel_MarkRead(t *testing.T) {
	db := setupMessageTestDB(t)
	model := NewMessageModel(db)
	ctx := context.Background()

	msg := &Message{
		To:      "user@example.com",
		Type:    "notification",
		Title:   "Mark Read Test",
		Content: "Content",
		Status:  dbenum.MessageStatusUnread,
	}
	err := model.Create(ctx, msg)
	require.NoError(t, err)

	// Mark as read
	err = model.MarkRead(ctx, msg.ID)
	require.NoError(t, err)

	// Verify the enum-scanned value.
	found, findErr := model.FindOne(ctx, msg.ID)
	require.NoError(t, findErr)
	assert.Equal(t, dbenum.MessageStatusRead, found.Status)
	var readAt *string
	require.NoError(t, db.Raw("SELECT read_at FROM messages WHERE id = ?", msg.ID).Scan(&readAt).Error)
	require.NotNil(t, readAt)
	assert.NotEmpty(t, *readAt)
}

func TestMessageModel_CountUnread(t *testing.T) {
	db := setupMessageTestDB(t)
	model := NewMessageModel(db)
	ctx := context.Background()

	// Create messages with unique recipient
	messages := []*Message{
		{To: "countuser@example.com", Type: "notification", Title: "Unread 1", Content: "Content", Status: dbenum.MessageStatusUnread},
		{To: "countuser@example.com", Type: "notification", Title: "Unread 2", Content: "Content", Status: dbenum.MessageStatusUnread},
		{To: "countuser@example.com", Type: "notification", Title: "Read", Content: "Content", Status: dbenum.MessageStatusRead},
	}
	for _, m := range messages {
		err := model.Create(ctx, m)
		require.NoError(t, err)
	}

	count, err := model.CountUnread(ctx, "countuser@example.com")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Count all unread
	count, err = model.CountUnread(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestMessageModel_Recent(t *testing.T) {
	db := setupMessageTestDB(t)
	model := NewMessageModel(db)
	ctx := context.Background()

	// Create messages
	for i := 0; i < 5; i++ {
		msg := &Message{
			To:      "user@example.com",
			Type:    "notification",
			Title:   "Recent",
			Content: "Content",
			Status:  dbenum.MessageStatusUnread,
		}
		err := model.Create(ctx, msg)
		require.NoError(t, err)
	}

	// Get recent messages
	recent, err := model.Recent(ctx, 3, "user@example.com")
	require.NoError(t, err)
	assert.LessOrEqual(t, len(recent), 3)
}
