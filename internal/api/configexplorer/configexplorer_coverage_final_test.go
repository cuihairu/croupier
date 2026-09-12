// 覆盖目标（coverage final）：
//  1. Service.List：src.List 返回错误（路径含 ".." 被 cleanPath 拒绝）
//     → 错误向上传播（service.go:112）。
//  2. Service.Write：可写源 ws.Write 失败（gorm create 回调对 config_versions
//     注入错误，版本写入失败）→ 错误向上传播（service.go:168）。
package configexplorer

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestList_SourceListError_RejectsTraversalDir(t *testing.T) {
	svcCtx := newTestEnvServiceContext(t)
	id := seedBinding(t, svcCtx.DB, "demo", "prod", "croupier-src", model.ConfigSourceTypeCroupier, `{}`)

	s := NewService(svcCtx)
	_, err := s.List(context.Background(), id, "../etc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path segment")
}

// explorerCovSchemaCache 供语句表名解析缓存复用。
var explorerCovSchemaCache = &sync.Map{}

// explorerCovTable 解析当前语句操作的物理表（显式 Table 优先，其次
// Model/Dest 的 schema）。
func explorerCovTable(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	for _, v := range []interface{}{tx.Statement.Model, tx.Statement.Dest} {
		if v == nil {
			continue
		}
		if s, err := schema.Parse(v, explorerCovSchemaCache, schema.NamingStrategy{}); err == nil {
			return s.Table
		}
	}
	return ""
}

func TestWrite_SourceWriteFails(t *testing.T) {
	svcCtx := newTestEnvServiceContext(t)
	db := svcCtx.DB
	id := seedBinding(t, db, "demo", "prod", "croupier-src", model.ConfigSourceTypeCroupier, `{}`)

	// croupier 源的应急编辑 = 注册新 ConfigVersion：对 config_versions 的
	// create 注入错误，模拟版本写入失败。
	require.NoError(t, db.Callback().Create().Before("gorm:create").
		Register("explorer_cov:write_fail", func(tx *gorm.DB) {
			if explorerCovTable(tx) == "config_versions" {
				_ = tx.AddError(errors.New("injected config version write failure"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("explorer_cov:write_fail") })

	s := NewService(svcCtx)
	err := s.Write(context.Background(), &WriteRequest{
		SourceID: id,
		Path:     "gameplay/k.json",
		Content:  `{"v":1}`,
		Reason:   "应急修正",
	}, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected config version write failure")
}
