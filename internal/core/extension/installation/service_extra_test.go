package installation

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInstallDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.ExtensionInstallation{},
		&model.ExtensionRuntimeBinding{},
		&model.ExtensionEvent{},
	))
	return db
}

func newInstallService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := setupInstallDB(t)
	return NewService(
		extensiongorm.NewInstallationRepo(db),
		extensiongorm.NewEventRepo(db),
		extensiongorm.NewBindingRepo(db),
	), db
}

func TestService_Install_Success(t *testing.T) {
	svc, db := newInstallService(t)
	ctx := context.Background()

	item, err := svc.Install(ctx, InstallRequest{
		ExtensionID:    "ext-1",
		ReleaseVersion: "1.0.0",
		ScopeType:      "game",
		ScopeID:        "game1",
		TargetType:     "agent",
		TargetID:       "agent-1",
		Config:         map[string]any{"k": "v"},
		SecretRefs:     map[string]string{"secret": "ref"},
		Operator:       "admin",
	})
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.NotZero(t, item.ID)
	assert.Equal(t, "installed", item.Status)
	assert.Equal(t, "disabled", item.DesiredState)
	assert.False(t, item.Enabled)
	assert.JSONEq(t, `{"k":"v"}`, string(item.ConfigJSON))
	assert.JSONEq(t, `{"secret":"ref"}`, string(item.SecretRefsJSON))
	assert.Equal(t, "admin", item.InstalledBy)

	// 安装事件被记录
	var events []model.ExtensionEvent
	require.NoError(t, db.Where("installation_id = ?", item.ID).Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, "install", events[0].EventType)
}

func TestService_Install_NilConfig(t *testing.T) {
	// 注意：nil 的 map 传入 marshalJSON(v any) 时 v != nil（带类型的 nil map），
	// 因此序列化结果为 "null" 而非 "{}"（见 bug 报告）。
	svc, _ := newInstallService(t)

	item, err := svc.Install(context.Background(), InstallRequest{ExtensionID: "ext-2"})
	require.NoError(t, err)
	assert.Equal(t, "null", string(item.ConfigJSON))
	assert.Equal(t, "null", string(item.SecretRefsJSON))
}

func TestService_Install_CreateError(t *testing.T) {
	svc, db := newInstallService(t)
	require.NoError(t, db.Migrator().DropTable(&model.ExtensionInstallation{}))

	_, err := svc.Install(context.Background(), InstallRequest{ExtensionID: "ext"})
	require.Error(t, err)
}

func TestService_List_WithData(t *testing.T) {
	svc, _ := newInstallService(t)
	ctx := context.Background()
	_, err := svc.Install(ctx, InstallRequest{ExtensionID: "ext-1", ScopeType: "game", ScopeID: "g1", TargetType: "agent", TargetID: "a1"})
	require.NoError(t, err)
	_, err = svc.Install(ctx, InstallRequest{ExtensionID: "ext-2", ScopeType: "game", ScopeID: "g2", TargetType: "global"})
	require.NoError(t, err)

	t.Run("list all", func(t *testing.T) {
		items, total, err := svc.List(ctx, ListQuery{})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("filter extension", func(t *testing.T) {
		items, total, err := svc.List(ctx, ListQuery{ExtensionID: "ext-1"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "ext-1", items[0].ExtensionID)
	})

	t.Run("filter scope", func(t *testing.T) {
		items, total, err := svc.List(ctx, ListQuery{ScopeType: "game", ScopeID: "g2"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "ext-2", items[0].ExtensionID)
	})

	t.Run("filter target", func(t *testing.T) {
		_, total, err := svc.List(ctx, ListQuery{TargetType: "global"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
	})

	t.Run("list error", func(t *testing.T) {
		svcErr, db := newInstallService(t)
		require.NoError(t, db.Migrator().DropTable(&model.ExtensionInstallation{}))
		_, _, err := svcErr.List(ctx, ListQuery{})
		require.Error(t, err)
	})
}

func TestService_Get_FoundAndMissing(t *testing.T) {
	svc, _ := newInstallService(t)
	ctx := context.Background()
	item, err := svc.Install(ctx, InstallRequest{ExtensionID: "ext-1"})
	require.NoError(t, err)

	got, err := svc.Get(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, item.ID, got.ID)

	missing, err := svc.Get(ctx, 9999)
	require.Error(t, err)
	assert.Nil(t, missing)
}

func TestService_UpdateConfig(t *testing.T) {
	svc, db := newInstallService(t)
	ctx := context.Background()
	item, err := svc.Install(ctx, InstallRequest{ExtensionID: "ext-1"})
	require.NoError(t, err)

	require.NoError(t, svc.UpdateConfig(ctx, item.ID, map[string]any{"a": 1}, map[string]string{"s": "r"}, "op"))

	got, err := svc.Get(ctx, item.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(got.ConfigJSON))
	assert.JSONEq(t, `{"s":"r"}`, string(got.SecretRefsJSON))

	var events []model.ExtensionEvent
	require.NoError(t, db.Where("installation_id = ? AND event_type = ?", item.ID, "update_config").Find(&events).Error)
	assert.Len(t, events, 1)
}

func TestService_UpdateConfig_Missing(t *testing.T) {
	svc, _ := newInstallService(t)
	err := svc.UpdateConfig(context.Background(), 9999, nil, nil, "op")
	require.Error(t, err)
}

func TestService_EnableDisable(t *testing.T) {
	svc, db := newInstallService(t)
	ctx := context.Background()
	item, err := svc.Install(ctx, InstallRequest{ExtensionID: "ext-1"})
	require.NoError(t, err)

	require.NoError(t, svc.Enable(ctx, item.ID, "op"))
	got, err := svc.Get(ctx, item.ID)
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, "enabled", got.Status)
	assert.Equal(t, "enabled", got.DesiredState)

	require.NoError(t, svc.Disable(ctx, item.ID, "op"))
	got, err = svc.Get(ctx, item.ID)
	require.NoError(t, err)
	assert.False(t, got.Enabled)
	assert.Equal(t, "disabled", got.Status)

	var events []model.ExtensionEvent
	require.NoError(t, db.Where("installation_id = ? AND event_type IN ?", item.ID, []string{"enable", "disable"}).Find(&events).Error)
	assert.Len(t, events, 2)
}

func TestService_EnableDisable_Missing(t *testing.T) {
	svc, _ := newInstallService(t)
	require.Error(t, svc.Enable(context.Background(), 9999, "op"))
	require.Error(t, svc.Disable(context.Background(), 9999, "op"))
}

func TestService_Upgrade(t *testing.T) {
	svc, db := newInstallService(t)
	ctx := context.Background()
	item, err := svc.Install(ctx, InstallRequest{ExtensionID: "ext-1", ReleaseVersion: "1.0.0"})
	require.NoError(t, err)

	require.NoError(t, svc.Upgrade(ctx, item.ID, "2.0.0", "op"))

	got, err := svc.Get(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", got.ReleaseVersion)
	assert.Equal(t, "enabled", got.Status)

	var events []model.ExtensionEvent
	require.NoError(t, db.Where("installation_id = ? AND event_type = ?", item.ID, "upgrade").Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, `2.0.0`, string(events[0].PayloadJSON))
}

func TestService_Upgrade_Missing(t *testing.T) {
	svc, _ := newInstallService(t)
	require.Error(t, svc.Upgrade(context.Background(), 9999, "2.0", "op"))
}

func TestService_Uninstall(t *testing.T) {
	svc, db := newInstallService(t)
	ctx := context.Background()
	item, err := svc.Install(ctx, InstallRequest{ExtensionID: "ext-1"})
	require.NoError(t, err)
	require.NoError(t, svc.Enable(ctx, item.ID, "op"))
	// 附加绑定，卸载时应清空
	require.NoError(t, svc.bindingRepo.ReplaceForInstallation(ctx, item.ID, []model.ExtensionRuntimeBinding{
		{BindingType: "t", BindingKey: "k", Status: "active"},
	}))

	require.NoError(t, svc.Uninstall(ctx, item.ID, "op"))

	got, err := svc.Get(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, "uninstalled", got.Status)
	assert.Equal(t, "uninstalled", got.DesiredState)
	assert.False(t, got.Enabled)

	bindings, err := svc.ListBindings(ctx, item.ID)
	require.NoError(t, err)
	assert.Empty(t, bindings)

	var events []model.ExtensionEvent
	require.NoError(t, db.Where("installation_id = ? AND event_type = ?", item.ID, "uninstall").Find(&events).Error)
	assert.Len(t, events, 1)
}

func TestService_Uninstall_Missing(t *testing.T) {
	svc, _ := newInstallService(t)
	require.Error(t, svc.Uninstall(context.Background(), 9999, "op"))
}

func TestService_ListEvents(t *testing.T) {
	svc, _ := newInstallService(t)
	ctx := context.Background()
	item, err := svc.Install(ctx, InstallRequest{ExtensionID: "ext-1"})
	require.NoError(t, err)
	require.NoError(t, svc.RecordEvent(ctx, item.ID, "custom", "warn", "warn message", "op", `{"p":1}`))

	t.Run("list all", func(t *testing.T) {
		events, total, err := svc.ListEvents(ctx, item.ID, EventListQuery{})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, events, 2)
	})

	t.Run("filter level", func(t *testing.T) {
		events, total, err := svc.ListEvents(ctx, item.ID, EventListQuery{Level: "warn"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "warn message", events[0].Message)
	})

	t.Run("filter keyword", func(t *testing.T) {
		_, total, err := svc.ListEvents(ctx, item.ID, EventListQuery{Keyword: "warn"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
	})

	t.Run("repo error", func(t *testing.T) {
		svcErr, db := newInstallService(t)
		require.NoError(t, db.Migrator().DropTable(&model.ExtensionEvent{}))
		_, _, err := svcErr.ListEvents(ctx, 1, EventListQuery{})
		require.Error(t, err)
	})
}

func TestService_ListBindings_WithData(t *testing.T) {
	svc, _ := newInstallService(t)
	ctx := context.Background()
	item, err := svc.Install(ctx, InstallRequest{ExtensionID: "ext-1"})
	require.NoError(t, err)
	require.NoError(t, svc.bindingRepo.ReplaceForInstallation(ctx, item.ID, []model.ExtensionRuntimeBinding{
		{BindingType: "t", BindingKey: "k1", Status: "active"},
	}))

	bindings, err := svc.ListBindings(ctx, item.ID)
	require.NoError(t, err)
	assert.Len(t, bindings, 1)
}
