package installation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// failUpdates 注册一个 update 回调，使所有 Save（UPDATE）失败而
// Create / Query 不受影响，用于覆盖各状态变更方法的 Save 错误分支。
func failUpdates(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:update_fail", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("save boom"))
	}))
}

func mustInstall(t *testing.T, svc *Service) uint {
	t.Helper()
	item, err := svc.Install(context.Background(), InstallRequest{ExtensionID: "ext-save-err", Operator: "admin"})
	require.NoError(t, err)
	require.NotZero(t, item.ID)
	return item.ID
}

func TestService_UpdateConfig_SaveError(t *testing.T) {
	svc, db := newInstallService(t)
	failUpdates(t, db)
	id := mustInstall(t, svc)

	err := svc.UpdateConfig(context.Background(), id, map[string]any{"k": "v"}, nil, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save boom")
}

func TestService_Upgrade_SaveError(t *testing.T) {
	svc, db := newInstallService(t)
	failUpdates(t, db)
	id := mustInstall(t, svc)

	err := svc.Upgrade(context.Background(), id, "2.0.0", "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save boom")
}

func TestService_Uninstall_SaveError(t *testing.T) {
	svc, db := newInstallService(t)
	failUpdates(t, db)
	id := mustInstall(t, svc)

	err := svc.Uninstall(context.Background(), id, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save boom")
}

func TestService_Enable_SaveError(t *testing.T) {
	svc, db := newInstallService(t)
	failUpdates(t, db)
	id := mustInstall(t, svc)

	err := svc.Enable(context.Background(), id, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save boom")
}

func TestService_Disable_SaveError(t *testing.T) {
	svc, db := newInstallService(t)
	failUpdates(t, db)
	id := mustInstall(t, svc)

	err := svc.Disable(context.Background(), id, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save boom")
}
