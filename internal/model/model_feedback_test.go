package model

import (
	"context"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewFeedbackModel(t *testing.T) {
	db := setupFeedbackTestDB(t)
	model := NewFeedbackModel(db)
	assert.NotNil(t, model)
}

func TestFeedbackModel_Create(t *testing.T) {
	db := setupFeedbackTestDB(t)
	model := NewFeedbackModel(db)
	ctx := context.Background()

	feedback := &Feedback{
		PlayerID: "player001",
		Contact:  "player@example.com",
		Content:  "Great game!",
		Category: "general",
		Priority: "low",
		Status:   dbenum.FeedbackStatusOpen,
		Rating:   5,
		GameID:   "game001",
		Env:      "prod",
	}

	err := model.Create(ctx, feedback)
	require.NoError(t, err)
	assert.NotZero(t, feedback.ID)
}

func TestFeedbackModel_Update(t *testing.T) {
	db := setupFeedbackTestDB(t)
	model := NewFeedbackModel(db)
	ctx := context.Background()

	feedback := &Feedback{
		PlayerID: "player001",
		Content:  "Content",
		Category: "bug",
		Status:   dbenum.FeedbackStatusOpen,
	}
	err := model.Create(ctx, feedback)
	require.NoError(t, err)

	// Update
	updates := map[string]interface{}{
		"status": dbenum.FeedbackStatusTriaged,
		"reply":  "Thank you for your feedback",
	}
	err = model.Update(ctx, feedback.ID, updates)
	require.NoError(t, err)

	// Verify
	found, err := model.FindByID(ctx, feedback.ID)
	require.NoError(t, err)
	assert.Equal(t, dbenum.FeedbackStatusTriaged, found.Status)
	assert.Equal(t, "Thank you for your feedback", found.Reply)
}

func TestFeedbackModel_Delete(t *testing.T) {
	db := setupFeedbackTestDB(t)
	model := NewFeedbackModel(db)
	ctx := context.Background()

	feedback := &Feedback{
		PlayerID: "player001",
		Content:  "Content",
		Category: "bug",
		Status:   dbenum.FeedbackStatusOpen,
	}
	err := model.Create(ctx, feedback)
	require.NoError(t, err)

	// Delete
	err = model.Delete(ctx, feedback.ID)
	require.NoError(t, err)

	// Verify
	_, err = model.FindByID(ctx, feedback.ID)
	assert.Error(t, err)
}

func TestFeedbackModel_FindByID(t *testing.T) {
	db := setupFeedbackTestDB(t)
	model := NewFeedbackModel(db)
	ctx := context.Background()

	feedback := &Feedback{
		PlayerID: "player001",
		Content:  "Find test",
		Category: "bug",
		Status:   dbenum.FeedbackStatusOpen,
	}
	err := model.Create(ctx, feedback)
	require.NoError(t, err)

	// Find existing
	found, err := model.FindByID(ctx, feedback.ID)
	require.NoError(t, err)
	assert.Equal(t, "Find test", found.Content)

	// Find non-existent
	_, err = model.FindByID(ctx, 99999)
	assert.Error(t, err)
}

func TestFeedbackModel_List(t *testing.T) {
	db := setupFeedbackTestDB(t)
	model := NewFeedbackModel(db)
	ctx := context.Background()

	feedbacks := []*Feedback{
		{PlayerID: "p1", Content: "Content 1", Category: "bug", Status: dbenum.FeedbackStatusOpen, GameID: "game1", Env: "prod"},
		{PlayerID: "p2", Content: "Content 2", Category: "feature", Status: dbenum.FeedbackStatusOpen, GameID: "game1", Env: "prod"},
		{PlayerID: "p3", Content: "Content 3", Category: "bug", Status: dbenum.FeedbackStatusTriaged, GameID: "game2", Env: "test"},
	}
	for _, f := range feedbacks {
		err := model.Create(ctx, f)
		require.NoError(t, err)
	}

	// List all
	_, total, err := model.List(ctx, ListFeedbackOptions{Status: -1, ExcludeStatus: -1})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))

	// Filter by game
	items, total, err := model.List(ctx, ListFeedbackOptions{GameID: "game1", Status: -1, ExcludeStatus: -1})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))
	_ = items // items is used for verification

	// Filter by status
	_, total, err = model.List(ctx, ListFeedbackOptions{Status: dbenum.FeedbackStatusOpen, ExcludeStatus: -1})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))

	// Filter by category
	_, total, err = model.List(ctx, ListFeedbackOptions{Category: "bug", Status: -1, ExcludeStatus: -1})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))

	// Filter by keyword
	_, total, err = model.List(ctx, ListFeedbackOptions{Keyword: "Content 1", Status: -1, ExcludeStatus: -1})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
}

func TestFeedbackModel_Stats(t *testing.T) {
	db := setupFeedbackTestDB(t)
	model := NewFeedbackModel(db)
	ctx := context.Background()

	feedbacks := []*Feedback{
		{PlayerID: "p1", Content: "Content 1", Category: "bug", Status: dbenum.FeedbackStatusOpen, Rating: 3},
		{PlayerID: "p2", Content: "Content 2", Category: "bug", Status: dbenum.FeedbackStatusOpen, Rating: 4},
		{PlayerID: "p3", Content: "Content 3", Category: "feature", Status: dbenum.FeedbackStatusTriaged, Rating: 5, Reply: "Thanks"},
	}
	for _, f := range feedbacks {
		err := model.Create(ctx, f)
		require.NoError(t, err)
	}

	// Get stats
	stats, err := model.Stats(ctx, FeedbackStatsOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.Total, int64(3))
	assert.NotEmpty(t, stats.ByCategory)
	assert.NotEmpty(t, stats.ByStatus)
	assert.Greater(t, stats.AvgRating, 0.0)
	assert.GreaterOrEqual(t, stats.Responded, int64(1)) // At least our 1 responded
}

// ===== FAQModel Tests =====
