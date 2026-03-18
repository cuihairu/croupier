package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSupportModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	assert.NotNil(t, model)
	assert.Same(t, db, model.db)
}

func TestSupportModel_CreateTicket(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	ticket := &SupportTicket{
		Title:    "Test Ticket",
		Content:  "Test content",
		Category: "bug",
		Priority: "high",
		Status:   "open",
		GameID:   "game1",
		Env:      "dev",
	}

	err := model.CreateTicket(ctx, ticket)
	assert.NoError(t, err)
	assert.NotZero(t, ticket.ID)
}

func TestSupportModel_UpdateTicket(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	ticket := &SupportTicket{
		Title:  "Test Ticket",
		Status: "open",
		GameID: "game1",
	}
	err := model.CreateTicket(ctx, ticket)
	assert.NoError(t, err)

	err = model.UpdateTicket(ctx, ticket.ID, map[string]interface{}{"status": "resolved"})
	assert.NoError(t, err)
}

func TestSupportModel_DeleteTicket(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	ticket := &SupportTicket{
		Title:  "Test Ticket",
		GameID: "game1",
	}
	err := model.CreateTicket(ctx, ticket)
	assert.NoError(t, err)

	err = model.DeleteTicket(ctx, ticket.ID)
	assert.NoError(t, err)
}

func TestSupportModel_ListTickets(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	// Clean up
	db.Exec("DELETE FROM support_tickets")

	tickets := []*SupportTicket{
		{Title: "Ticket 1", Status: "open", GameID: "game1"},
		{Title: "Ticket 2", Status: "closed", GameID: "game1"},
		{Title: "Ticket 3", Status: "open", GameID: "game1"},
	}

	for _, ticket := range tickets {
		err := model.CreateTicket(ctx, ticket)
		assert.NoError(t, err)
	}

	// List all
	all, total, err := model.ListTickets(ctx, ListTicketsOptions{})
	assert.NoError(t, err)
	assert.Len(t, all, 3)
	assert.Equal(t, int64(3), total)

	// Filter by status
	open, total, err := model.ListTickets(ctx, ListTicketsOptions{Status: "open"})
	assert.NoError(t, err)
	assert.Len(t, open, 2)
	assert.Equal(t, int64(2), total)
}

func TestSupportModel_CreateComment(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	ticket := &SupportTicket{Title: "Test Ticket", GameID: "game1"}
	err := model.CreateTicket(ctx, ticket)
	assert.NoError(t, err)

	comment := &SupportComment{
		TicketID: ticket.ID,
		Author:   "admin",
		Content:  "Test comment",
	}

	err = model.CreateComment(ctx, comment)
	assert.NoError(t, err)
	assert.NotZero(t, comment.ID)
}

func TestSupportModel_ListComments(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	ticket := &SupportTicket{Title: "Test Ticket", GameID: "game1"}
	err := model.CreateTicket(ctx, ticket)
	assert.NoError(t, err)

	comments := []*SupportComment{
		{TicketID: ticket.ID, Author: "admin", Content: "Comment 1"},
		{TicketID: ticket.ID, Author: "user", Content: "Comment 2"},
	}

	for _, c := range comments {
		err := model.CreateComment(ctx, c)
		assert.NoError(t, err)
	}

	result, err := model.ListComments(ctx, ticket.ID)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestSupportModel_CreateFAQ(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	faq := &SupportFAQ{
		Question: "What is this?",
		Answer:   "This is a test",
		Category: "general",
		Visible:  true,
		Sort:     1,
	}

	err := model.CreateFAQ(ctx, faq)
	assert.NoError(t, err)
	assert.NotZero(t, faq.ID)
}

func TestSupportModel_ListFAQs(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	// Clean up
	db.Exec("DELETE FROM support_faqs")

	faqs := []*SupportFAQ{
		{Question: "Q1", Answer: "A1", Category: "general", Sort: 2},
		{Question: "Q2", Answer: "A2", Category: "technical", Sort: 1},
	}

	for _, f := range faqs {
		err := model.CreateFAQ(ctx, f)
		assert.NoError(t, err)
	}

	result, err := model.ListFAQs(ctx)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	// Should be ordered by sort DESC
	assert.Equal(t, "Q1", result[0].Question)
}

func TestSupportModel_CreateFeedback(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	feedback := &SupportFeedback{
		PlayerID: "player1",
		Content:  "Great service",
		Category: "praise",
		GameID:   "game1",
		Env:      "prod",
	}

	err := model.CreateFeedback(ctx, feedback)
	assert.NoError(t, err)
	assert.NotZero(t, feedback.ID)
}

func TestSupportModel_ListFeedback(t *testing.T) {
	db := setupTestDB(t)
	model := NewSupportModel(db)
	ctx := context.Background()

	// Clean up
	db.Exec("DELETE FROM support_feedback")

	feedbacks := []*SupportFeedback{
		{PlayerID: "player1", Content: "Good", Category: "praise"},
		{PlayerID: "player2", Content: "Bad", Category: "complaint"},
	}

	for _, f := range feedbacks {
		err := model.CreateFeedback(ctx, f)
		assert.NoError(t, err)
	}

	result, err := model.ListFeedback(ctx)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
}
