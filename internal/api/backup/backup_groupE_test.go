// 覆盖目标：backup service.saveBackupsToExtensionInstallation 的
// CurrentUsername 命中分支（service.go:347-349）。
//
// 该函数的 operator 默认 "system"，仅当 ctx 注入非空 username 时改写；
// 现有用例均以 context.Background() 调用，本文件注入 username 并回读
// 校验落库内容。
package backup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveBackupsToExtensionInstallation_WithUsernameGroupE(t *testing.T) {
	env := setupBackupFlowEnv(t)
	installBackupAdvancedExtension(t, env, map[string]any{})

	ctx := context.WithValue(context.Background(), "username", "alice")
	require.NoError(t, env.service.saveBackupsToExtensionInstallation(ctx, []Backup{
		{Id: "bkp-groupE-1", Name: "one", Type: "database", Status: "completed"},
	}))

	items, ok, err := env.service.loadBackupsFromExtensionInstallation(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "bkp-groupE-1", items[0].Id)
	assert.Equal(t, "one", items[0].Name)
}
