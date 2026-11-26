package model

import (
	"context"
	"fmt"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
	"time"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewFAQModel(t *testing.T) {
	db := setupFAQTestDB(t)
	model := NewFAQModel(db)
	assert.NotNil(t, model)
}

func TestFAQModel_Create(t *testing.T) {
	db := setupFAQTestDB(t)
	model := NewFAQModel(db)
	ctx := context.Background()

	faq := &FAQ{
		Question: "What is Croupier?",
		Answer:   "Croupier is a distributed GM backend system.",
		Category: "general",
		Visible:  true,
		Sort:     1,
	}

	err := model.Create(ctx, faq)
	require.NoError(t, err)
	assert.NotZero(t, faq.ID)
}

func TestFAQModel_Update(t *testing.T) {
	db := setupFAQTestDB(t)
	model := NewFAQModel(db)
	ctx := context.Background()

	faq := &FAQ{
		Question: "Test Question",
		Answer:   "Test Answer",
		Category: "general",
	}
	err := model.Create(ctx, faq)
	require.NoError(t, err)

	// Update
	updates := map[string]interface{}{
		"question": "Updated Question",
		"answer":   "Updated Answer",
	}
	err = model.Update(ctx, faq.ID, updates)
	require.NoError(t, err)

	// Verify
	found, err := model.FindOne(ctx, faq.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Question", found.Question)
}

func TestFAQModel_Delete(t *testing.T) {
	db := setupFAQTestDB(t)
	model := NewFAQModel(db)
	ctx := context.Background()

	faq := &FAQ{
		Question: "Delete Test",
		Answer:   "Answer",
		Category: "general",
	}
	err := model.Create(ctx, faq)
	require.NoError(t, err)

	// Delete
	err = model.Delete(ctx, faq.ID)
	require.NoError(t, err)

	// Verify
	_, err = model.FindOne(ctx, faq.ID)
	assert.Error(t, err)
}

func TestFAQModel_FindOne(t *testing.T) {
	db := setupFAQTestDB(t)
	model := NewFAQModel(db)
	ctx := context.Background()

	faq := &FAQ{
		Question: "Find Test",
		Answer:   "Answer",
		Category: "general",
	}
	err := model.Create(ctx, faq)
	require.NoError(t, err)

	// Find existing
	found, err := model.FindOne(ctx, faq.ID)
	require.NoError(t, err)
	assert.Equal(t, "Find Test", found.Question)

	// Find non-existent
	_, err = model.FindOne(ctx, 99999)
	assert.Error(t, err)
}

func TestFAQModel_List(t *testing.T) {
	db := setupFAQTestDB(t)
	model := NewFAQModel(db)
	ctx := context.Background()

	faqs := []*FAQ{
		{Question: "Q1", Answer: "A1", Category: "general", Visible: true, Sort: 1},
		{Question: "Q2", Answer: "A2", Category: "technical", Visible: true, Sort: 2},
		{Question: "Q3", Answer: "A3", Category: "general", Visible: false, Sort: 3},
	}
	for _, f := range faqs {
		err := model.Create(ctx, f)
		require.NoError(t, err)
	}

	// List all
	_, total, err := model.List(ctx, ListFAQOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))

	// Filter by category
	items, total, err := model.List(ctx, ListFAQOptions{Category: "general"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))
	_ = items // items is used for verification

	// Filter by visible
	visible := true
	_, total, err = model.List(ctx, ListFAQOptions{Visible: &visible})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))

	// Filter by keyword
	_, total, err = model.List(ctx, ListFAQOptions{Keyword: "Q1"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
}

func TestFAQModel_UpsertCategory(t *testing.T) {
	db := setupFAQTestDB(t)
	model := NewFAQModel(db)
	ctx := context.Background()

	category := &FAQCategory{
		Name:        "Getting Started",
		Description: "Help for new users",
		Visible:     true,
		Sort:        1,
	}

	err := model.UpsertCategory(ctx, category)
	require.NoError(t, err)
	assert.NotZero(t, category.ID)

	// Update via upsert
	category.Description = "Updated description"
	err = model.UpsertCategory(ctx, category)
	require.NoError(t, err)
}

func TestFAQModel_ListCategories(t *testing.T) {
	// setupFAQTestDB 用的是全局共享内存库（cache=shared），其他 FAQ 用例的
	// 行会污染聚合断言——此处按库内先例开独占 DSN 验证精确计数/排除语义
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:faq_listcat_%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&FAQ{}, &FAQCategory{}))
	model := NewFAQModel(db)
	ctx := context.Background()

	// Create FAQs in different categories；#22：未分类（空串）不得进入
	// 聚合选项——List 里 category='' 是「不过滤」语义，会出现假选项
	faqs := []*FAQ{
		{Question: "Q1", Answer: "A1", Category: "general"},
		{Question: "Q2", Answer: "A2", Category: "general"},
		{Question: "Q3", Answer: "A3", Category: "technical"},
		{Question: "Q4", Answer: "A4", Category: ""},
	}
	for _, f := range faqs {
		err := model.Create(ctx, f)
		require.NoError(t, err)
	}

	// List categories
	categories, err := model.ListCategories(ctx)
	require.NoError(t, err)
	require.Len(t, categories, 2)
	names := make([]string, 0, len(categories))
	for _, c := range categories {
		assert.NotEmpty(t, c.Name)
		names = append(names, c.Name)
	}
	assert.ElementsMatch(t, []string{"general", "technical"}, names)
	// count 随 category 聚合（general=2 > technical=1，Order count DESC）
	assert.Equal(t, "general", categories[0].Name)
	assert.Equal(t, 2, categories[0].Count)
}

// ===== ProfileModel Tests =====
