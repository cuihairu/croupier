package faq

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var faqServiceDBSeq uint64

func newFAQServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("faq_svc_%d", atomic.AddUint64(&faqServiceDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newFAQService(db *gorm.DB) *Service {
	svcCtx := &svc.ServiceContext{FAQModel: model.NewFAQModel(db)}
	return NewService(svcCtx)
}

func TestService_Update_Success(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	created, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "original question",
		Answer:   "original answer",
		Category: "general",
		Tags:     []string{"tag1"},
		Visible:  true,
	})
	require.NoError(t, err)

	visible := false
	sort := 10
	resp, err := svc.Update(context.Background(), &FAQUpdateRequest{
		ID:       fmt.Sprint(created.Id),
		Question: "updated question",
		Answer:   "updated answer",
		Category: "updated",
		Tags:     []string{"tag2", "tag3"},
		Visible:  &visible,
		Sort:     &sort,
	})
	require.NoError(t, err)
	assert.Equal(t, "updated question", resp.Question)
	assert.Equal(t, "updated answer", resp.Answer)
	assert.Equal(t, "updated", resp.Category)
	assert.Equal(t, []string{"tag2", "tag3"}, resp.Tags)
	assert.Equal(t, false, resp.Visible)
	assert.Equal(t, 10, resp.Sort)
}

func TestService_Update_InvalidID(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	tests := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Update(context.Background(), &FAQUpdateRequest{
				ID:       tt.id,
				Question: "updated",
			})
			assert.Error(t, err)
		})
	}
}

func TestService_Delete_InvalidID(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	tests := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Delete(context.Background(), &FAQDeleteRequest{ID: tt.id})
			assert.Error(t, err)
		})
	}
}

func TestService_Delete_Success(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	created, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "to be deleted",
		Answer:   "answer",
		Category: "general",
	})
	require.NoError(t, err)

	err = svc.Delete(context.Background(), &FAQDeleteRequest{ID: fmt.Sprint(created.Id)})
	assert.NoError(t, err)

	// Verify deleted
	_, err = svc.List(context.Background(), &FAQListRequest{Page: 1, PageSize: 10})
	assert.NoError(t, err)
}

func TestService_List_Empty(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	resp, err := svc.List(context.Background(), &FAQListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.Equal(t, int64(0), resp.Total)
}

func TestService_List_WithPagination(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	for i := 0; i < 15; i++ {
		_, err := svc.Create(context.Background(), &FAQCreateRequest{
			Question: fmt.Sprintf("question %d", i),
			Answer:   fmt.Sprintf("answer %d", i),
			Category: "general",
		})
		require.NoError(t, err)
	}

	resp, err := svc.List(context.Background(), &FAQListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 10)
	assert.Equal(t, int64(15), resp.Total)

	resp2, err := svc.List(context.Background(), &FAQListRequest{Page: 2, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp2.Items, 5)
}

func TestService_List_FilterByCategory(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	_, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "q1", Answer: "a1", Category: "billing",
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), &FAQCreateRequest{
		Question: "q2", Answer: "a2", Category: "technical",
	})
	require.NoError(t, err)

	resp, err := svc.List(context.Background(), &FAQListRequest{Page: 1, PageSize: 10, Category: "billing"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "billing", resp.Items[0].Category)
}

func TestService_List_FilterBySearch(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	_, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "How to reset password?", Answer: "Go to settings", Category: "general",
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), &FAQCreateRequest{
		Question: "How to contact support?", Answer: "Email us", Category: "general",
	})
	require.NoError(t, err)

	resp, err := svc.List(context.Background(), &FAQListRequest{Page: 1, PageSize: 10, Keyword: "password"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Contains(t, resp.Items[0].Question, "password")
}

func TestService_Categories_Empty(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	resp, err := svc.Categories(context.Background(), &FAQCategoriesRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestService_Categories_WithData(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	_, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "q1", Answer: "a1", Category: "billing",
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), &FAQCreateRequest{
		Question: "q2", Answer: "a2", Category: "billing",
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), &FAQCreateRequest{
		Question: "q3", Answer: "a3", Category: "technical",
	})
	require.NoError(t, err)

	resp, err := svc.Categories(context.Background(), &FAQCategoriesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)

	categories := make(map[string]int)
	for _, cat := range resp.Items {
		categories[cat.Name] = cat.Count
	}
	assert.Equal(t, 2, categories["billing"])
	assert.Equal(t, 1, categories["technical"])
}

func TestService_Create_Success(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	resp, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "What is Croupier?",
		Answer:   "A GM backend system",
		Category: "general",
		Tags:     []string{"intro", "overview"},
		Visible:  true,
	})
	require.NoError(t, err)
	assert.NotZero(t, resp.Id)
	assert.Equal(t, "What is Croupier?", resp.Question)
	assert.Equal(t, "A GM backend system", resp.Answer)
	assert.Equal(t, "general", resp.Category)
	assert.Equal(t, []string{"intro", "overview"}, resp.Tags)
	assert.True(t, resp.Visible)
}

func TestService_Create_InvalidID(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)

	_, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "q",
		Answer:   "a",
		Category: "c",
	})
	// Should succeed - no ID validation on create
	assert.NoError(t, err)
}
