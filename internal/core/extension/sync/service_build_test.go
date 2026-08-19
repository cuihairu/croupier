package sync

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

func setupSyncDB(t *testing.T) *gorm.DB {
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

func newSyncService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := setupSyncDB(t)
	return NewService(extensiongorm.NewInstallationRepo(db), extensiongorm.NewBindingRepo(db)), db
}

func TestBuildAgentPayload_NilRepos(t *testing.T) {
	ctx := context.Background()

	t.Run("nil service", func(t *testing.T) {
		var s *Service
		_, err := s.BuildAgentPayload(ctx, "agent-1")
		assert.ErrorIs(t, err, gorm.ErrInvalidDB)
	})
	t.Run("nil installation repo", func(t *testing.T) {
		s := NewService(nil, nil)
		_, err := s.BuildAgentPayload(ctx, "agent-1")
		assert.ErrorIs(t, err, gorm.ErrInvalidDB)
	})
}

func TestBuildAgentPayload_FiltersAndBindings(t *testing.T) {
	svc, _ := newSyncService(t)
	ctx := context.Background()

	matched := &model.ExtensionInstallation{
		InstallationKey: "k-agent",
		ExtensionID:     "ext-a",
		ReleaseVersion:  "1.0.0",
		ScopeType:       "game",
		ScopeID:         "game1",
		TargetType:      "agent",
		TargetID:        "agent-1",
		Status:          "installed",
		DesiredState:    "installed",
		Enabled:         true,
		ConfigJSON:      []byte(`{"a":1}`),
		SecretRefsJSON:  []byte(`{}`),
	}
	require.NoError(t, svc.installationRepo.Create(ctx, matched))
	require.NoError(t, svc.bindingRepo.ReplaceForInstallation(ctx, matched.ID, []model.ExtensionRuntimeBinding{
		{BindingType: "binding_type", BindingKey: "bk-1", TargetRef: "ref-1", SpecJSON: []byte(`{"x":1}`), Status: "active"},
	}))

	// 状态为 uninstalled 的应被过滤
	uninstalled := &model.ExtensionInstallation{
		InstallationKey: "k-uninstalled",
		ExtensionID:     "ext-b",
		TargetType:      "agent",
		TargetID:        "agent-1",
		Status:          "uninstalled",
	}
	require.NoError(t, svc.installationRepo.Create(ctx, uninstalled))

	// desiredState 为 uninstalled 的也应被过滤
	desiredUninstall := &model.ExtensionInstallation{
		InstallationKey: "k-desired",
		ExtensionID:     "ext-c",
		TargetType:      "agent",
		TargetID:        "agent-1",
		Status:          "installed",
		DesiredState:    "uninstalled",
	}
	require.NoError(t, svc.installationRepo.Create(ctx, desiredUninstall))

	// 目标 agent 不匹配的应被过滤
	otherAgent := &model.ExtensionInstallation{
		InstallationKey: "k-other",
		ExtensionID:     "ext-d",
		TargetType:      "agent",
		TargetID:        "agent-2",
		Status:          "installed",
	}
	require.NoError(t, svc.installationRepo.Create(ctx, otherAgent))

	// group 目标匹配所有 agent
	groupItem := &model.ExtensionInstallation{
		InstallationKey: "k-group",
		ExtensionID:     "ext-e",
		TargetType:      "agent_group",
		TargetID:        "default",
		Status:          "installed",
	}
	require.NoError(t, svc.installationRepo.Create(ctx, groupItem))

	payload, err := svc.BuildAgentPayload(ctx, "agent-1")
	require.NoError(t, err)
	require.NotNil(t, payload)
	assert.Equal(t, "agent-1", payload.AgentID)
	assert.NotZero(t, payload.GeneratedAt)
	assert.NotEmpty(t, payload.Version)
	require.Len(t, payload.Installations, 2)

	var byExt = map[string]AgentInstallationPayload{}
	for _, inst := range payload.Installations {
		byExt[inst.ExtensionID] = inst
	}

	a := byExt["ext-a"]
	assert.Equal(t, "k-agent", a.InstallationKey)
	assert.Equal(t, "1.0.0", a.ReleaseVersion)
	assert.True(t, a.Enabled)
	assert.Equal(t, `{"a":1}`, a.ConfigJSON)
	require.Len(t, a.Bindings, 1)
	assert.Equal(t, "bk-1", a.Bindings[0].BindingKey)
	assert.Equal(t, "active", a.Bindings[0].Status)

	assert.Contains(t, byExt, "ext-e")
	assert.NotContains(t, byExt, "ext-b")
	assert.NotContains(t, byExt, "ext-c")
	assert.NotContains(t, byExt, "ext-d")
}

func TestBuildAgentPayload_EmptyList(t *testing.T) {
	svc, _ := newSyncService(t)

	payload, err := svc.BuildAgentPayload(context.Background(), "agent-1")
	require.NoError(t, err)
	require.NotNil(t, payload)
	assert.Empty(t, payload.Installations)
}

func TestBuildAgentPayload_ListError(t *testing.T) {
	svc, db := newSyncService(t)
	require.NoError(t, db.Migrator().DropTable(&model.ExtensionInstallation{}))

	_, err := svc.BuildAgentPayload(context.Background(), "agent-1")
	require.Error(t, err)
}

func TestBuildAgentPayload_BindingListError(t *testing.T) {
	svc, db := newSyncService(t)
	ctx := context.Background()
	require.NoError(t, svc.installationRepo.Create(ctx, &model.ExtensionInstallation{
		InstallationKey: "k-err",
		ExtensionID:     "ext-x",
		TargetType:      "global",
		Status:          "installed",
	}))
	require.NoError(t, db.Migrator().DropTable(&model.ExtensionRuntimeBinding{}))

	_, err := svc.BuildAgentPayload(ctx, "agent-1")
	require.Error(t, err)
}
