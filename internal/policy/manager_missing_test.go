package policy

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 配置文件全部不可读：回落到内置默认策略。
func TestNewManager_NoConfigFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	origWd := testWorkingDir(t)
	require.NoError(t, testChdir(t, dir))
	t.Cleanup(func() { _ = testChdir(t, origWd) })

	m, err := NewManager(setupTestDBExtra(t), filepath.Join(dir, "no-such.yaml"))
	require.NoError(t, err)
	require.NotNil(t, m.config)
	assert.False(t, m.config.Low.RequireApproval)
	assert.True(t, m.config.High.RequireApproval)
}

// ListOverrides 查询失败：drop 表后返回错误。
func TestListOverrides_DBError(t *testing.T) {
	m, err := NewManager(setupTestDBExtra(t), filepath.Join(t.TempDir(), "no-such.yaml"))
	require.NoError(t, err)
	require.NoError(t, m.db.Migrator().DropTable("function_policies"))

	_, err = m.ListOverrides(t.Context())
	require.Error(t, err)
}

// GetPolicy：manual 查询直接报错（非 RecordNotFound）→ 透传错误。
func TestGetPolicy_FirstQueryError(t *testing.T) {
	m, err := NewManager(setupTestDBExtra(t), filepath.Join(t.TempDir(), "no-such.yaml"))
	require.NoError(t, err)
	require.NoError(t, m.db.Migrator().DropTable("function_policies"))

	_, err = m.GetPolicy(t.Context(), "f1", RiskHigh)
	require.Error(t, err)
}
