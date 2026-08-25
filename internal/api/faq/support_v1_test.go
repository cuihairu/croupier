package faq

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedFAQ(t *testing.T, svc *Service, question, slug string, tags []string) int64 {
	t.Helper()
	created, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: question,
		Answer:   "answer for " + question,
		Category: "general",
		Tags:     tags,
		Visible:  true,
		Slug:     slug,
		Summary:  "summary for " + question,
	})
	require.NoError(t, err)
	return created.Id
}

func TestService_Vote_Counters(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	id := seedFAQ(t, svc, "how to recharge", "recharge", nil)

	vote := func(helpful bool) *FAQVoteResponse {
		resp, err := svc.Vote(context.Background(), &FAQVoteRequest{ID: fmt.Sprint(id), Helpful: helpful})
		require.NoError(t, err)
		return resp
	}
	assert.Equal(t, 1, vote(true).HelpfulCount)
	assert.Equal(t, 2, vote(true).HelpfulCount)
	assert.Equal(t, 1, vote(false).UnhelpfulCount)

	// Counters are also visible through List responses.
	list, err := svc.List(context.Background(), &FAQListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, 2, list.Items[0].HelpfulCount)
	assert.Equal(t, 1, list.Items[0].UnhelpfulCount)
	assert.Equal(t, "recharge", list.Items[0].Slug)
	assert.Equal(t, "summary for how to recharge", list.Items[0].Summary)
}

func TestService_Vote_NotFound(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	_, err := svc.Vote(context.Background(), &FAQVoteRequest{ID: "999", Helpful: true})
	require.Error(t, err)
}

func TestService_List_TagFilter(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	seedFAQ(t, svc, "recharge issue", "", []string{"payment"})
	seedFAQ(t, svc, "login issue", "", []string{"account"})

	resp, err := svc.List(context.Background(), &FAQListRequest{Tag: "payment"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Contains(t, resp.Items[0].Tags, "payment")
}

func TestService_List_OrderByHelpful(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	hot := seedFAQ(t, svc, "hot question", "", nil)
	cold := seedFAQ(t, svc, "cold question", "", nil)
	for range 5 {
		_, err := svc.Vote(context.Background(), &FAQVoteRequest{ID: fmt.Sprint(hot), Helpful: true})
		require.NoError(t, err)
	}
	_, err := svc.Vote(context.Background(), &FAQVoteRequest{ID: fmt.Sprint(cold), Helpful: false})
	require.NoError(t, err)

	resp, err := svc.List(context.Background(), &FAQListRequest{OrderBy: "helpful"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, hot, resp.Items[0].Id)
}

func TestService_SlugUniqueness(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	seedFAQ(t, svc, "first", "dup-slug", nil)

	// Create with a taken slug conflicts.
	_, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "second", Answer: "a", Category: "general", Slug: "dup-slug",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug")

	// Update to a taken slug conflicts; to a fresh one succeeds.
	second := seedFAQ(t, svc, "second", "", nil)
	taken := "dup-slug"
	_, err = svc.Update(context.Background(), &FAQUpdateRequest{ID: fmt.Sprint(second), Slug: &taken})
	require.Error(t, err)
	fresh := "fresh-slug"
	_, err = svc.Update(context.Background(), &FAQUpdateRequest{ID: fmt.Sprint(second), Slug: &fresh})
	require.NoError(t, err)

	// Empty slugs are allowed repeatedly (pre-slug rows).
	seedFAQ(t, svc, "third", "", nil)
	seedFAQ(t, svc, "fourth", "", nil)
}

func TestModel_Vote_AtomicAndMissing(t *testing.T) {
	db := newFAQServiceTestDB(t)
	m := model.NewFAQModel(db)
	faq := &model.FAQ{Question: "q", Answer: "a", Category: "c", Visible: true}
	require.NoError(t, m.Create(context.Background(), faq))

	require.NoError(t, m.Vote(context.Background(), faq.ID, true))
	require.NoError(t, m.Vote(context.Background(), faq.ID, true))
	require.NoError(t, m.Vote(context.Background(), faq.ID, false))
	stored, err := m.FindOne(context.Background(), faq.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, stored.HelpfulCount)
	assert.Equal(t, 1, stored.UnhelpfulCount)

	err = m.Vote(context.Background(), 424242, true)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
