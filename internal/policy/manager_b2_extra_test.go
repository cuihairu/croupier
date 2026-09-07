package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetPolicy_DefaultRowScanError 注入 created_at 非法的 default 行：
// manual 查询仍返回 ErrRecordNotFound，default 查询扫描失败，
// 覆盖 GetPolicy 第二次查询返回非 ErrRecordNotFound 错误的分支。
func TestGetPolicy_DefaultRowScanError(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)

	require.NoError(t, db.Exec(
		`INSERT INTO function_policies (function_id, source, require_approval, created_at, updated_at)
		 VALUES ('fn.b2', 'default', 'zzz', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)

	_, err = m.GetPolicy(context.Background(), "fn.b2", RiskLow)
	assert.Error(t, err)
}
