// Coverage target: the remaining uncovered branches of Vote/Update in service.go --
// 1) FindOne fails after a successful Vote write (service.go:61-63);
// 2) the Summary pointer field branch of Update (service.go:142-144);
// 3) FindOne fails after a successful Update write (service.go:155-157).
// FAQModel is a concrete type (not an interface), so this reuses the package's
// existing "in-memory sqlite + fault injection" approach: a gorm Query callback
// injects an error so SELECT (FindOne) fails deterministically while
// UPDATE/CREATE still succeed, precisely triggering the "write ok, read-back
// failed" error path.
package faq

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var errFAQCovFindOne = errors.New("faqCov: find_one fault injected")

// faqCovInjectFindOneError registers a gorm query callback that fails every
// SELECT on db while leaving write statements (UPDATE/CREATE/DELETE) intact.
func faqCovInjectFindOneError(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("faq_cov:fail_query", func(tx *gorm.DB) {
		_ = tx.AddError(errFAQCovFindOne)
	}))
}

func TestFAQCov_Vote_FindOneAfterVoteError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	id := seedFAQ(t, svc, "vote then read fails", "vote-cov", nil)

	faqCovInjectFindOneError(t, db)

	_, err := svc.Vote(context.Background(), &FAQVoteRequest{ID: fmt.Sprint(id), Helpful: true})
	require.ErrorIs(t, err, errFAQCovFindOne)
}

func TestFAQCov_Update_FindOneAfterUpdateError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	id := seedFAQ(t, svc, "update then read fails", "update-cov", nil)

	faqCovInjectFindOneError(t, db)

	_, err := svc.Update(context.Background(), &FAQUpdateRequest{ID: fmt.Sprint(id), Question: "q2"})
	require.ErrorIs(t, err, errFAQCovFindOne)
}

func TestFAQCov_Update_Summary(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	id := seedFAQ(t, svc, "summary target", "summary-cov", nil)

	summary := "  trimmed summary  "
	resp, err := svc.Update(context.Background(), &FAQUpdateRequest{ID: fmt.Sprint(id), Summary: &summary})
	require.NoError(t, err)
	assert.Equal(t, "trimmed summary", resp.Summary)

	empty := "   "
	resp, err = svc.Update(context.Background(), &FAQUpdateRequest{ID: fmt.Sprint(id), Summary: &empty})
	require.NoError(t, err)
	assert.Equal(t, "", resp.Summary)
}
