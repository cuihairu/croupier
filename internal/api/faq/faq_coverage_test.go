// 覆盖目标：service.go 中 Vote/Update 的剩余未覆盖分支——
// 1) Vote 成功写库后 FindOne 失败（service.go:61-63）；
// 2) Update 的 Summary 指针字段分支（service.go:142-144）；
// 3) Update 成功写库后 FindOne 失败（service.go:155-157）。
// FAQModel 为具体类型（非接口），沿用包内既有"内存 sqlite + 故障注入"方案：
// 这里通过 gorm Query 回调注入错误，使 SELECT（FindOne）确定性失败而
// UPDATE/CREATE 仍成功，从而精确触发"写库成功、回读失败"的错误路径。
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

var faqCovFindOneErr = errors.New("faqCov: find_one fault injected")

// faqCovInjectFindOneError registers a gorm query callback that fails every
// SELECT on db while leaving write statements (UPDATE/CREATE/DELETE) intact.
func faqCovInjectFindOneError(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("faq_cov:fail_query", func(tx *gorm.DB) {
		_ = tx.AddError(faqCovFindOneErr)
	}))
}

func TestFAQCov_Vote_FindOneAfterVoteError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	id := seedFAQ(t, svc, "vote then read fails", "vote-cov", nil)

	faqCovInjectFindOneError(t, db)

	_, err := svc.Vote(context.Background(), &FAQVoteRequest{ID: fmt.Sprint(id), Helpful: true})
	require.ErrorIs(t, err, faqCovFindOneErr)
}

func TestFAQCov_Update_FindOneAfterUpdateError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	id := seedFAQ(t, svc, "update then read fails", "update-cov", nil)

	faqCovInjectFindOneError(t, db)

	_, err := svc.Update(context.Background(), &FAQUpdateRequest{ID: fmt.Sprint(id), Question: "q2"})
	require.ErrorIs(t, err, faqCovFindOneErr)
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
