// 覆盖目标（组 E）：
//   - probe.Probe 的 sql.Open 失败分支（probe.go:68-71）
//   - service.UpdateSource 的更新后重读失败分支（service.go:103-105）
package dbmon

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 无 "/" 的 mysql DSN 在 go-sql-driver 的 OpenConnector 阶段即被拒绝
// （ParseDSN: missing the slash），sql.Open 返回错误 → 探测结果承载
// open 错误而非接口报错。
func TestProbe_SQLOpenInvalidDSNGroupE(t *testing.T) {
	src := &model.DBSource{Model: gorm.Model{ID: 1}, Name: "bad-dsn", Driver: "mysql", Kind: model.DBSourceKindSelf}

	res, err := Probe(context.Background(), src, "groupE-no-slash-dsn")

	require.NoError(t, err, "probe failures are reported on the result")
	require.NotNil(t, res)
	assert.False(t, res.OK)
	assert.Contains(t, res.Error, "open:")
	assert.NotZero(t, res.SourceID)
	assert.Equal(t, "mysql", res.Driver)
}

// UpdateSource 流程中的 query 序列：FindOne(#1) → Update → FindOne(#2)。
// 注入第 2 次 query 起失败的回调，覆盖更新成功后重读失败的分支。
func TestService_UpdateSource_ReloadErrorGroupE(t *testing.T) {
	dbmonSvc, srcModel, _, db := newDBMonDBFixture(t)
	ctx := context.Background()
	src := &model.DBSource{
		Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf,
		DSN: goodDSN, Enabled: true,
	}
	require.NoError(t, srcModel.Create(ctx, src))

	var n int
	require.NoError(t, db.Callback().Query().Before("gorm:query").
		Register("groupE_fail_query", func(tx *gorm.DB) {
			n++
			if n >= 2 {
				_ = tx.AddError(errors.New("forced reload failure"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("groupE_fail_query") })

	resp, err := dbmonSvc.UpdateSource(ctx, &SourceUpdateRequest{
		ID:   fmt.Sprintf("%d", src.ID),
		Name: "s2",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced reload failure")
	assert.Nil(t, resp)
}
