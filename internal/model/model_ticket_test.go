package model

import (
	"context"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewTicketModel(t *testing.T) {
	db := setupTicketTestDB(t)
	model := NewTicketModel(db)
	assert.NotNil(t, model)
}

func TestTicketModel_Create(t *testing.T) {
	db := setupTicketTestDB(t)
	model := NewTicketModel(db)
	ctx := context.Background()

	ticket := &Ticket{
		Title:    "Test Ticket",
		Content:  "Test ticket content",
		Category: "bug",
		Priority: "high",
		Status:   dbenum.TicketStatusOpen,
		Assignee: "admin",
		GameID:   "game001",
		Env:      "prod",
	}

	err := model.Create(ctx, ticket)
	require.NoError(t, err)
	assert.NotZero(t, ticket.ID)
}

func TestTicketModel_Update(t *testing.T) {
	db := setupTicketTestDB(t)
	model := NewTicketModel(db)
	ctx := context.Background()

	ticket := &Ticket{
		Title:    "Update Test",
		Content:  "Content",
		Category: "bug",
		Status:   dbenum.TicketStatusOpen,
	}
	err := model.Create(ctx, ticket)
	require.NoError(t, err)

	// Update
	updates := map[string]interface{}{
		"status":   dbenum.TicketStatusInProgress,
		"assignee": "developer",
	}
	err = model.Update(ctx, ticket.ID, updates)
	require.NoError(t, err)

	// Verify
	found, err := model.FindOne(ctx, ticket.ID)
	require.NoError(t, err)
	assert.Equal(t, dbenum.TicketStatusInProgress, found.Status)
	assert.Equal(t, "developer", found.Assignee)
}

func TestTicketModel_Delete(t *testing.T) {
	db := setupTicketTestDB(t)
	model := NewTicketModel(db)
	ctx := context.Background()

	ticket := &Ticket{
		Title:    "Delete Test",
		Content:  "Content",
		Category: "bug",
		Status:   dbenum.TicketStatusOpen,
	}
	err := model.Create(ctx, ticket)
	require.NoError(t, err)

	// Delete
	err = model.Delete(ctx, ticket.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = model.FindOne(ctx, ticket.ID)
	assert.Error(t, err)
}

func TestTicketModel_FindOne(t *testing.T) {
	db := setupTicketTestDB(t)
	model := NewTicketModel(db)
	ctx := context.Background()

	ticket := &Ticket{
		Title:    "Find One Test",
		Content:  "Content",
		Category: "bug",
		Status:   dbenum.TicketStatusOpen,
	}
	err := model.Create(ctx, ticket)
	require.NoError(t, err)

	// Find existing
	found, err := model.FindOne(ctx, ticket.ID)
	require.NoError(t, err)
	assert.Equal(t, "Find One Test", found.Title)

	// Find non-existent
	_, err = model.FindOne(ctx, 99999)
	assert.Error(t, err)
}

func TestTicketModel_List(t *testing.T) {
	db := setupTicketTestDB(t)
	model := NewTicketModel(db)
	ctx := context.Background()

	tickets := []*Ticket{
		{Title: "Ticket 1", Category: "bug", Priority: "high", Status: dbenum.TicketStatusOpen, Assignee: "admin"},
		{Title: "Ticket 2", Category: "feature", Priority: "low", Status: dbenum.TicketStatusOpen, Assignee: "user"},
		{Title: "Ticket 3", Category: "bug", Priority: "medium", Status: dbenum.TicketStatusClosed, Assignee: "admin"},
	}
	for _, tkt := range tickets {
		err := model.Create(ctx, tkt)
		require.NoError(t, err)
	}

	// List all：Status 零值恰为 TicketStatusOpen(0)，List 把 >=0 当作启用
	// 过滤（-1 才是无过滤哨兵，注释见 TicketQueryOptions）；裸查询会在干净
	// 库上只剩 2 条 open（Ticket3 是 closed），此前靠同包共享库残留行凑够
	// 断言，-shuffle=on 下先跑即挂（CI 实证 2026-11-12）。
	_, total, err := model.List(ctx, TicketQueryOptions{Status: -1})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))

	// Filter by status
	items, total, err := model.List(ctx, TicketQueryOptions{Status: dbenum.TicketStatusOpen})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))
	_ = items // items is used for verification

	// Filter by category（无 status 过滤，closed 的 Ticket3 也计入）
	_, total, err = model.List(ctx, TicketQueryOptions{Category: "bug", Status: -1})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))

	// Filter by assignee（同上，跨状态计数）
	_, total, err = model.List(ctx, TicketQueryOptions{Assignee: "admin", Status: -1})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))
}

func TestTicketModel_CreateComment(t *testing.T) {
	db := setupTicketTestDB(t)
	model := NewTicketModel(db)
	ctx := context.Background()

	ticket := &Ticket{
		Title:    "Comment Test",
		Content:  "Content",
		Category: "bug",
		Status:   dbenum.TicketStatusOpen,
	}
	err := model.Create(ctx, ticket)
	require.NoError(t, err)

	comment := &TicketComment{
		TicketID: ticket.ID,
		Author:   "admin",
		Content:  "Test comment",
	}

	err = model.CreateComment(ctx, comment)
	require.NoError(t, err)
	assert.NotZero(t, comment.ID)
}

func TestTicketModel_ListComments(t *testing.T) {
	db := setupTicketTestDB(t)
	model := NewTicketModel(db)
	ctx := context.Background()

	ticket := &Ticket{
		Title:    "List Comments Test",
		Content:  "Content",
		Category: "bug",
		Status:   dbenum.TicketStatusOpen,
	}
	err := model.Create(ctx, ticket)
	require.NoError(t, err)

	comments := []*TicketComment{
		{TicketID: ticket.ID, Author: "user1", Content: "First comment"},
		{TicketID: ticket.ID, Author: "user2", Content: "Second comment"},
	}
	for _, c := range comments {
		err := model.CreateComment(ctx, c)
		require.NoError(t, err)
	}

	// List comments
	list, err := model.ListComments(ctx, ticket.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
}

// ===== FeedbackModel Tests =====
