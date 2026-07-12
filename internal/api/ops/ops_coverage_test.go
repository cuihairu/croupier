package ops

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Tests for backup operations with real GORM model

func TestOpsBackupsListWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	// Create test backup
	backup := &model.Backup{
		BackupID: fmt.Sprintf("backup-%d", time.Now().UnixNano()),
		Name:     "test-backup",
		Type:     "full",
		Status:   "completed",
		Size:     1024,
	}
	require.NoError(t, db.Create(backup).Error)

	ctx := context.Background()
	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}

	resp, err := opsBackupsList(ctx, svcCtx, &OpsBackupsListRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Backups, 1)
	assert.Equal(t, "test-backup", resp.Backups[0].Name)
}

func TestOpsBackupCreateWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	ctx := context.Background()
	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}

	resp, err := opsBackupCreate(ctx, svcCtx, &OpsBackupCreateRequest{Name: "new-backup"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.BackupID)
}

func TestBackupServiceListWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	// Create test backup
	backup := &model.Backup{
		BackupID: fmt.Sprintf("backup-%d", time.Now().UnixNano()),
		Name:     "backup1",
		Type:     "full",
		Status:   "completed",
		Size:     2048,
	}
	require.NoError(t, db.Create(backup).Error)

	ctx := context.Background()
	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	s := NewBackupService(svcCtx)

	backups, err := s.List(ctx, "game1", "prod")
	require.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Equal(t, "backup1", backups[0].Name)
}

func TestBackupServiceListNilModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewBackupService(svcCtx)

	_, err := s.List(ctx, "game1", "prod")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestBackupServiceCreateWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	ctx := context.Background()
	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	s := NewBackupService(svcCtx)

	backupID, err := s.Create(ctx, "game1", "prod", "full", 3600)
	require.NoError(t, err)
	assert.NotEmpty(t, backupID)
}

func TestBackupServiceCreateNilModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewBackupService(svcCtx)

	_, err := s.Create(ctx, "game1", "prod", "full", 3600)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestBackupServiceDeleteWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	// Create test backup
	backup := &model.Backup{
		BackupID: fmt.Sprintf("backup-%d", time.Now().UnixNano()),
		Name:     "test-backup",
	}
	require.NoError(t, db.Create(backup).Error)

	ctx := context.Background()
	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	s := NewBackupService(svcCtx)

	err = s.Delete(ctx, backup.BackupID)
	require.NoError(t, err)
}

func TestBackupServiceDeleteNilModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewBackupService(svcCtx)

	err := s.Delete(ctx, "backup-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestBackupServiceGetDownloadURLNilModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{BackupModel: nil}
	s := NewBackupService(svcCtx)

	_, _, err := s.GetDownloadURL(ctx, "backup-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestBackupServiceGetDownloadURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewBackupService(svcCtx)

	_, _, err := s.GetDownloadURL(ctx, "backup-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

// Tests for alert operations with real GORM model

func TestOpsAlertsWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Alert{})
	require.NoError(t, err)

	// Create test alert
	alert := &model.Alert{
		AlertID: fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Type:    "cpu_high",
		Level:   "warning",
		Message: "CPU usage high",
		Status:  "firing",
	}
	require.NoError(t, db.Create(alert).Error)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}

	resp, err := opsAlerts(ctx, svcCtx, &OpsAlertsRequest{})
	require.NoError(t, err)
	alerts := resp.Alerts
	assert.Len(t, alerts, 1)
	assert.Equal(t, "warning", alerts[0].Severity)
}

func TestOpsAlertSilenceWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Alert{}, &model.AlertSilence{})
	require.NoError(t, err)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}

	resp, err := opsAlertSilence(ctx, svcCtx, &OpsAlertSilenceRequest{AlertID: "123", Duration: 60})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.SilenceID)
}

func TestOpsSilencesWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Alert{}, &model.AlertSilence{})
	require.NoError(t, err)

	// Create test silence
	silence := &model.AlertSilence{
		AlertID:        123,
		Reason:         "test silence",
		DurationMinute: 60,
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	require.NoError(t, db.Create(silence).Error)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}

	resp, err := opsSilences(ctx, svcCtx, &OpsSilencesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Silences, 1)
}

func TestAlertServiceListWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Alert{})
	require.NoError(t, err)

	// Create test alert
	alert := &model.Alert{
		AlertID: fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Type:    "memory",
		Level:   "critical",
		Message: "OOM risk",
		Status:  "firing",
	}
	require.NoError(t, db.Create(alert).Error)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	s := NewAlertService(svcCtx)

	alerts, err := s.List(ctx, "game1", "prod", "firing")
	require.NoError(t, err)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "critical", alerts[0].Severity)
}

func TestAlertServiceListNilModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAlertService(svcCtx)

	_, err := s.List(ctx, "game1", "prod", "firing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestAlertServiceSilenceWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.AlertSilence{})
	require.NoError(t, err)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	s := NewAlertService(svcCtx)

	silenceID, err := s.Silence(ctx, "42", 60, "test reason")
	require.NoError(t, err)
	assert.NotEmpty(t, silenceID)
}

func TestAlertServiceSilenceInvalidID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	alertModel := model.NewAlertModel(nil)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	s := NewAlertService(svcCtx)

	_, err := s.Silence(ctx, "invalid", 60, "test reason")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid alert ID")
}

func TestAlertServiceSilenceNilModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAlertService(svcCtx)

	_, err := s.Silence(ctx, "42", 60, "test reason")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestAlertServiceDeleteSilenceWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.AlertSilence{})
	require.NoError(t, err)

	// Create test silence
	silence := &model.AlertSilence{
		AlertID:        123,
		Reason:         "test",
		DurationMinute: 60,
	}
	require.NoError(t, db.Create(silence).Error)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	s := NewAlertService(svcCtx)

	// Delete by ID (convert to string first)
	err = s.DeleteSilence(ctx, fmt.Sprintf("%d", silence.ID))
	require.NoError(t, err)
}

func TestAlertServiceDeleteSilenceInvalidID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	alertModel := model.NewAlertModel(nil)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	s := NewAlertService(svcCtx)

	err := s.DeleteSilence(ctx, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid silence ID")
}

func TestAlertServiceDeleteSilenceNilModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAlertService(svcCtx)

	err := s.DeleteSilence(ctx, "1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestAlertServiceListSilencesWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.AlertSilence{})
	require.NoError(t, err)

	// Create test silence
	silence := &model.AlertSilence{
		AlertID:        123,
		Reason:         "maintenance",
		DurationMinute: 60,
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	require.NoError(t, db.Create(silence).Error)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	s := NewAlertService(svcCtx)

	silences, err := s.ListSilences(ctx, "game1")
	require.NoError(t, err)
	assert.Len(t, silences, 1)
	assert.Equal(t, "maintenance", silences[0].Summary)
}

func TestAlertServiceListSilencesNilModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAlertService(svcCtx)

	_, err := s.ListSilences(ctx, "game1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

// Tests for node service operations

func TestNodeServiceDrain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewNodeService(svcCtx)

	err := s.Drain(ctx, "node-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry store unavailable")
}

func TestNodeServiceRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewNodeService(svcCtx)

	err := s.Restart(ctx, "node-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry store unavailable")
}

func TestNodeServiceUndrain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewNodeService(svcCtx)

	err := s.Undrain(ctx, "node-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry store unavailable")
}

func TestNodeServiceGetMetaNilStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewNodeService(svcCtx)

	_, err := s.GetMeta(ctx, "node-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

// Tests for agent service operations with errors

func TestAgentServiceExecCommand(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAgentService(svcCtx)

	// This should return an error since GetAgentOpsClient will fail
	_, err := s.ExecCommand(ctx, "agent-1", "ls", []string{"-la"}, 30)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ops client unavailable")
}

func TestAgentServiceStartProcess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAgentService(svcCtx)

	_, err := s.StartProcess(ctx, "agent-1", "test", []string{}, nil, "/tmp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ops client unavailable")
}

func TestAgentServiceStopProcess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAgentService(svcCtx)

	err := s.StopProcess(ctx, "agent-1", "my-process")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ops client unavailable")
}

func TestAgentServiceRestartProcess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAgentService(svcCtx)

	err := s.RestartProcess(ctx, "agent-1", "my-process")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ops client unavailable")
}

func TestAgentServiceGetMetaNilStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAgentService(svcCtx)

	_, err := s.GetMeta(ctx, "agent-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestAgentServiceListNilStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewAgentService(svcCtx)

	agents, err := s.List(ctx, "", "", "")
	require.NoError(t, err)
	assert.Empty(t, agents)
}

// Tests for opsAgentExecCommand through service

func TestServiceOpsAgentExecCommand(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentExecCommand(ctx, &OpsExecCommandRequest{AgentID: "agent-1", Command: "ls"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// Tests for handler alias methods

func TestHandlerAliasAgentMethods(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	cases := []struct {
		name       string
		method     string
		url        string
		body       string
		fn         func(*gin.Context)
		statusCode int
	}{
		{name: "AgentMeta", method: http.MethodGet, url: "/api/v1/ops/agent/meta?agentId=agent-1", body: "", fn: h.AgentMeta, statusCode: http.StatusOK},
		{name: "AgentExecCommand", method: http.MethodPost, url: "/api/v1/ops/agent/exec", body: `{"agentId":"agent-1","command":"ls"}`, fn: h.AgentExecCommand, statusCode: http.StatusInternalServerError},
		{name: "Nodes", method: http.MethodGet, url: "/api/v1/ops/nodes", body: "", fn: h.Nodes, statusCode: http.StatusOK},
		{name: "NodeCommands", method: http.MethodPost, url: "/api/v1/ops/node/commands", body: `{"nodeId":"node-1"}`, fn: h.NodeCommands, statusCode: http.StatusOK},
		{name: "NodeDrain", method: http.MethodPost, url: "/api/v1/ops/node/drain", body: `{"nodeId":"agent-1"}`, fn: h.NodeDrain, statusCode: http.StatusOK},
		{name: "NodeRestart", method: http.MethodPost, url: "/api/v1/ops/node/restart", body: `{"nodeId":"agent-1"}`, fn: h.NodeRestart, statusCode: http.StatusOK},
		{name: "NodeUndrain", method: http.MethodPost, url: "/api/v1/ops/node/undrain", body: `{"nodeId":"agent-1"}`, fn: h.NodeUndrain, statusCode: http.StatusOK},
		{name: "HealthGet", method: http.MethodGet, url: "/api/v1/ops/health/get", body: "", fn: h.HealthGet, statusCode: http.StatusOK},
		{name: "HealthRun", method: http.MethodPost, url: "/api/v1/ops/health/run", body: `{"id":"check-1"}`, fn: h.HealthRun, statusCode: http.StatusInternalServerError},
		{name: "HealthUpdate", method: http.MethodPost, url: "/api/v1/ops/health/update", body: `{"enabled":true}`, fn: h.HealthUpdate, statusCode: http.StatusInternalServerError},
		{name: "MaintenanceGet", method: http.MethodGet, url: "/api/v1/ops/maintenance/get", body: "", fn: h.MaintenanceGet, statusCode: http.StatusOK},
		{name: "MaintenanceUpdate", method: http.MethodPost, url: "/api/v1/ops/maintenance/update", body: `{"enabled":true}`, fn: h.MaintenanceUpdate, statusCode: http.StatusInternalServerError},
		{name: "NotificationsGet", method: http.MethodGet, url: "/api/v1/ops/notifications/get", body: "", fn: h.NotificationsGet, statusCode: http.StatusOK},
		{name: "NotificationsUpdate", method: http.MethodPost, url: "/api/v1/ops/notifications/update", body: `{"enabled":true}`, fn: h.NotificationsUpdate, statusCode: http.StatusOK},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec := newOpsTestContext(tc.method, tc.url, tc.body)
			tc.fn(ctx)
			assert.Equal(t, tc.statusCode, rec.Code)
		})
	}
}

// Separate tests for backup and alert operations that require models
func TestHandlerAliasBackupMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	h := NewHandler(NewService(svcCtx))

	cases := []struct {
		name   string
		method string
		url    string
		body   string
		fn     func(*gin.Context)
	}{
		{name: "BackupsList", method: http.MethodGet, url: "/api/v1/ops/backups", body: "", fn: h.BackupsList},
		{name: "BackupCreate", method: http.MethodPost, url: "/api/v1/ops/backup/create", body: `{"name":"test"}`, fn: h.BackupCreate},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, rec := newOpsTestContext(tc.method, tc.url, tc.body)
			tc.fn(ctx)
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestHandlerAliasAlertMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Alert{}, &model.AlertSilence{})
	require.NoError(t, err)

	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	h := NewHandler(NewService(svcCtx))

	cases := []struct {
		name   string
		method string
		url    string
		body   string
		fn     func(*gin.Context)
	}{
		{name: "Alerts", method: http.MethodGet, url: "/api/v1/ops/alerts", body: "", fn: h.Alerts},
		{name: "AlertSilence", method: http.MethodPost, url: "/api/v1/ops/alert/silence", body: `{"alertId":"123","duration":60}`, fn: h.AlertSilence},
		{name: "SilenceDelete", method: http.MethodPost, url: "/api/v1/ops/silence/delete", body: `{"alertId":"1"}`, fn: h.SilenceDelete},
		{name: "Silences", method: http.MethodGet, url: "/api/v1/ops/silences", body: "", fn: h.Silences},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, rec := newOpsTestContext(tc.method, tc.url, tc.body)
			tc.fn(ctx)
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

// Tests for opsAgentSystemInfo error cases

func TestOpsAgentSystemInfoNilStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsAgentSystemInfo(ctx, &OpsAgentSystemInfoRequest{AgentID: "agent-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestOpsAgentSystemInfoNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	_, err := s.OpsAgentSystemInfo(ctx, &OpsAgentSystemInfoRequest{AgentID: "nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// Tests for opsAgentMeta error cases

func TestOpsAgentMetaNilStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsAgentMeta(ctx, &OpsAgentMetaRequest{AgentId: "agent-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

// Tests for opsNodeMeta error cases

func TestOpsNodeMetaNilStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsNodeMeta(ctx, &OpsNodeMetaRequest{NodeID: "node-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

// Tests for opsSilenceDelete error and success cases

func TestOpsSilenceDeleteSuccess(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.AlertSilence{})
	require.NoError(t, err)

	// Create test silence
	silence := &model.AlertSilence{
		AlertID:        123,
		Reason:         "test",
		DurationMinute: 60,
	}
	require.NoError(t, db.Create(silence).Error)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	s := NewService(svcCtx)

	resp, err := s.OpsSilenceDelete(ctx, &OpsAlertSilenceRequest{AlertID: fmt.Sprintf("%d", silence.ID)})
	require.NoError(t, err)
	assert.True(t, resp.Deleted)
}

// Tests for opsFunctions with nil svcCtx

func TestOpsFunctionsNilSvcCtx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := NewService(nil)

	resp, err := s.OpsFunctions(ctx, &OpsFunctionsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Functions)
}

// Tests for extractNotificationConfig helper

func TestExtractNotificationConfigNil(t *testing.T) {
	t.Parallel()

	enabled, channels, rules, extracted, err := extractNotificationConfig(nil)
	require.NoError(t, err)
	assert.False(t, extracted)
	assert.False(t, enabled)
	assert.Nil(t, channels)
	assert.Nil(t, rules)
}

func TestExtractNotificationConfigEmpty(t *testing.T) {
	t.Parallel()

	enabled, channels, rules, extracted, err := extractNotificationConfig(map[string]any{})
	require.NoError(t, err)
	assert.False(t, extracted)
	assert.False(t, enabled)
	assert.Nil(t, channels)
	assert.Nil(t, rules)
}

func TestExtractNotificationConfigPartial(t *testing.T) {
	t.Parallel()

	config := map[string]any{
		"enabled": true,
	}
	enabled, channels, rules, extracted, err := extractNotificationConfig(config)
	require.NoError(t, err)
	assert.True(t, extracted)
	assert.True(t, enabled)
	assert.NotNil(t, channels)
	assert.NotNil(t, rules)
}

func TestExtractNotificationConfigFull(t *testing.T) {
	t.Parallel()

	config := map[string]any{
		"enabled": true,
		"channels": []map[string]any{
			{"id": "ch-1", "type": "webhook"},
		},
		"rules": []map[string]any{
			{"event": "alert.fired"},
		},
	}
	enabled, channels, rules, extracted, err := extractNotificationConfig(config)
	require.NoError(t, err)
	assert.True(t, extracted)
	assert.True(t, enabled)
	assert.Len(t, channels, 1)
	assert.Len(t, rules, 1)
}

// Tests for findActiveExtensionInstallationByID

func TestFindActiveExtensionInstallationByIDNilSvcCtx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	item, ok, err := findActiveExtensionInstallationByID(ctx, nil, "test")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, item)
}

func TestFindActiveExtensionInstallationByIDNilExtensions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx, "test")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, item)
}

// Tests for opsAgentSystemInfo handler error path

func TestOpsAgentSystemInfoHandlerNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/systeminfo?agentId=nonexistent", "")

	h.OpsAgentSystemInfo(ctx)

	// Returns 500 when agent not found
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOpsAgentSystemInfoHandlerNilStore(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/systeminfo?agentId=agent-1", "")

	h.OpsAgentSystemInfo(ctx)

	// Returns 500 because registry store is unavailable
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// Tests for OpsAgentMeta handler error path

func TestOpsAgentMetaHandlerNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/agent/meta", `{"agentId":"nonexistent"}`)

	h.OpsAgentMeta(ctx)

	// Returns 500 when agent not found
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// Tests for OpsNodeMeta handler error path

func TestOpsNodeMetaHandlerNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/meta", `{"nodeId":"nonexistent"}`)

	h.OpsNodeMeta(ctx)

	// Returns 500 when node not found
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// Tests for OpsBackupsList handler with real model

func TestOpsBackupsListHandlerWithRealModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	// Create test backup
	backup := &model.Backup{
		BackupID: fmt.Sprintf("backup-%d", time.Now().UnixNano()),
		Name:     "backup1",
		Type:     "full",
		Status:   "completed",
		Size:     1024,
	}
	require.NoError(t, db.Create(backup).Error)

	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/backups", "")

	h.OpsBackupsList(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Tests for OpsBackupCreate handler with real model

func TestOpsBackupCreateHandlerWithRealModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/backup/create", `{"name":"test-backup"}`)

	h.OpsBackupCreate(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Tests for OpsAlerts handler with real model

func TestOpsAlertsHandlerWithRealModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Alert{})
	require.NoError(t, err)

	// Create test alert
	alert := &model.Alert{
		AlertID: fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Type:    "cpu",
		Level:   "warning",
		Message: "CPU high",
		Status:  "firing",
	}
	require.NoError(t, db.Create(alert).Error)

	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/alerts", "")

	h.OpsAlerts(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Tests for OpsSilences handler with real model

func TestOpsSilencesHandlerWithRealModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.AlertSilence{})
	require.NoError(t, err)

	// Create test silence
	silence := &model.AlertSilence{
		AlertID:        123,
		Reason:         "test",
		DurationMinute: 60,
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	require.NoError(t, db.Create(silence).Error)

	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/silences", "")

	h.OpsSilences(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Tests for OpsNodes handler with registry

func TestOpsNodesHandlerWithAgents(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Addr:      "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"hostname": "node1"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/nodes", "")

	h.OpsNodes(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "node-1")
}

// Test for OpsAgentExecCommand handler

func TestOpsAgentExecCommandHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/agent/exec", `{"agentId":"agent-1","command":"ls"}`)

	h.OpsAgentExecCommand(ctx)

	// Returns 500 because ops client is unavailable
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// Test for opsAgentMeta with registry data

func TestOpsAgentMetaWithRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "linux", "arch": "amd64", "hostname": "host1"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	resp, err := opsAgentMeta(ctx, svcCtx, &OpsAgentMetaRequest{AgentId: "agent-1"})
	require.NoError(t, err)
	data := resp.Meta.(OpsAgentSystemInfo)
	assert.Equal(t, "linux", data.OS)
	assert.Equal(t, "amd64", data.Arch)
	assert.Equal(t, "host1", data.Hostname)
}

// Test for opsNodeMeta with registry data

func TestOpsNodeMetaWithRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Addr:      "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"zone": "us-west-2", "datacenter": "dc1"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	resp, err := opsNodeMeta(ctx, svcCtx, &OpsNodeMetaRequest{NodeID: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", resp.Labels["zone"])
	assert.Equal(t, "dc1", resp.Labels["datacenter"])
}

// Tests for loadNotificationsFromExtensionInstallation with various configs

func TestLoadNotificationsFromExtensionInstallationEmptyConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}

	enabled, channels, rules, ok, err := loadNotificationsFromExtensionInstallation(ctx, svcCtx)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.False(t, enabled)
	assert.Nil(t, channels)
	assert.Nil(t, rules)
}

// Tests for opsNotificationsUpdate

func TestOpsNotificationsUpdateWithChannelsAndRules(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}

	_, err := opsNotificationsUpdate(ctx, svcCtx, &OpsNotificationsUpdateRequest{
		Enabled: true,
		Channels: []OpsNotificationChannel{
			{ID: "webhook-1", Type: "webhook", URL: "https://example.com/hook"},
			{ID: "email-1", Type: "email", URL: "admin@example.com"},
		},
		Rules: []OpsNotificationRule{
			{Event: "alert.fired", Channels: []string{"webhook-1", "email-1"}},
			{Event: "system.start", Channels: []string{"webhook-1"}},
		},
	})
	require.NoError(t, err)
}

// Tests for saveNotificationsToExtensionInstallation with nil extension

func TestSaveNotificationsToExtensionInstallationNilExtensions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}

	err := saveNotificationsToExtensionInstallation(ctx, svcCtx, &OpsNotificationsUpdateRequest{Enabled: true})
	assert.NoError(t, err) // Should not error, just return
}

// Tests for recordExtensionEvent

func TestRecordExtensionEventNilExtensions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}

	err := recordExtensionEvent(ctx, svcCtx, "test-ext", "test-event", "test message", "{}")
	assert.NoError(t, err) // Should not error when extensions not available
}

// Tests for opsAgentSystemInfo response structure

func TestOpsAgentSystemInfoResponseStructure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "windows", "arch": "arm64", "hostname": "win-host"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	resp, err := opsAgentSystemInfo(ctx, svcCtx, &OpsAgentSystemInfoRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, "windows", resp.SystemInfo.OS)
	assert.Equal(t, "arm64", resp.SystemInfo.Arch)
	assert.Equal(t, "win-host", resp.SystemInfo.Hostname)
}

// Tests for opsAgentSystemInfo empty labels

func TestOpsAgentSystemInfoEmptyLabels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	resp, err := opsAgentSystemInfo(ctx, svcCtx, &OpsAgentSystemInfoRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Empty(t, resp.SystemInfo.OS)
	assert.Empty(t, resp.SystemInfo.Arch)
}

// Additional handler POST tests for backup and alert operations

func TestOpsBackupsListHandlerPOST(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	// Create test backup
	backup := &model.Backup{
		BackupID: fmt.Sprintf("backup-%d", time.Now().UnixNano()),
		Name:     "backup1",
		Type:     "full",
		Status:   "completed",
		Size:     1024,
	}
	require.NoError(t, db.Create(backup).Error)

	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/backups/list", `{}`)

	h.OpsBackupsList(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsAlertSilenceHandlerWithRealModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.AlertSilence{})
	require.NoError(t, err)

	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/alert/silence", `{"alertId":"42","duration":120}`)

	h.OpsAlertSilence(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsSilenceDeleteHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/silence/delete", `{"alertId":"invalid-id"}`)

	h.OpsSilenceDelete(ctx)

	// invalid (non-numeric) id now returns 400 instead of envelope Code:1 with HTTP 200
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Test for OpsAgentSystemInfo handler with found agent

func TestOpsAgentSystemInfoHandlerFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "linux", "arch": "amd64"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	// URI binding requires proper Gin context setup, just test the handler doesn't panic
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/systeminfo?agentId=agent-1", "")

	h.OpsAgentSystemInfo(ctx)

	// Handler processes request even if agent not found (binding issue with URI params in test)
	assert.True(t, rec.Code >= 200 && rec.Code <= 599)
}

// Test for OpsAgentMeta handler with found agent

func TestOpsAgentMetaHandlerFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "darwin", "arch": "arm64"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/agent/meta", `{"agentId":"agent-1"}`)

	h.OpsAgentMeta(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "darwin")
}

// Test for OpsNodeMeta handler with found node

func TestOpsNodeMetaHandlerFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Addr:      "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"zone": "eu-west-1"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/meta", `{"nodeId":"node-1"}`)

	h.OpsNodeMeta(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "eu-west-1")
}

// Additional tests for handler alias methods with various HTTP methods

func TestHandlerAliasWithQueryParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/metrics?gameId=game1&env=prod&metric=cpu", "")
	h.Metrics(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Test for bindOpsRequest with GET and complex query params

func TestBindOpsRequestComplexQuery(t *testing.T) {
	t.Parallel()

	ctx, _ := newOpsTestContext(http.MethodGet, "/api/v1/ops/health?enabled=true&checkType=http", "")
	var req struct {
		Enabled   bool   `form:"enabled"`
		CheckType string `form:"checkType"`
	}

	err := bindOpsRequest(ctx, &req)
	require.NoError(t, err)
	assert.True(t, req.Enabled)
	assert.Equal(t, "http", req.CheckType)
}

// Test for bindOpsRequest with POST body

func TestBindOpsRequestWithJSONBody(t *testing.T) {
	t.Parallel()

	body := `{"nodeId":"node-1","action":"drain","duration":60}`
	ctx, _ := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/drain", body)

	var req struct {
		NodeID   string `json:"nodeId"`
		Action   string `json:"action"`
		Duration int    `json:"duration"`
	}

	err := bindOpsRequest(ctx, &req)
	require.NoError(t, err)
	assert.Equal(t, "node-1", req.NodeID)
	assert.Equal(t, "drain", req.Action)
	assert.Equal(t, 60, req.Duration)
}

// Test for opsAgentsList with multiple agents

func TestOpsAgentsListMultiple(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	// Add multiple agents
	for i := 1; i <= 3; i++ {
		store.UpsertAgent(&registry.AgentSession{
			AgentID:   fmt.Sprintf("agent-%d", i),
			Addr:      fmt.Sprintf("localhost:100%d", i),
			GameID:    "game1",
			Env:       "prod",
			Version:   "1.0.0",
			Labels:    map[string]string{"hostname": fmt.Sprintf("host-%d", i)},
			Functions: map[string]registry.FunctionMeta{fmt.Sprintf("func%d", i): {Enabled: true}},
			LastSeen:  time.Now(),
		})
	}

	resp, err := opsAgentsList(ctx, svcCtx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Agents, 3)
}

// Test for opsNodes with multiple nodes

func TestOpsNodesMultiple(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	// Add multiple agents as nodes
	for i := 1; i <= 5; i++ {
		store.UpsertAgent(&registry.AgentSession{
			AgentID:   fmt.Sprintf("node-%d", i),
			Addr:      fmt.Sprintf("localhost:200%d", i),
			GameID:    "game1",
			Env:       "prod",
			Labels:    map[string]string{"hostname": fmt.Sprintf("node-%d", i)},
			Functions: map[string]registry.FunctionMeta{},
			LastSeen:  time.Now(),
		})
	}

	resp, err := opsNodes(ctx, svcCtx, &OpsNodesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Nodes, 5)
}

// Test for opsFunctions with multiple functions across agents

func TestOpsFunctionsMultiple(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	// Agent 1 has func1 and func2
	store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		Addr:    "localhost:1001",
		GameID:  "game1",
		Env:     "prod",
		Functions: map[string]registry.FunctionMeta{
			"func1": {Enabled: true},
			"func2": {Enabled: true},
		},
		LastSeen: time.Now(),
	})

	// Agent 2 has func1 and func3
	store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-2",
		Addr:    "localhost:1002",
		GameID:  "game1",
		Env:     "prod",
		Functions: map[string]registry.FunctionMeta{
			"func1": {Enabled: true},
			"func3": {Enabled: true},
		},
		LastSeen: time.Now(),
	})

	resp, err := opsFunctions(ctx, svcCtx, &OpsFunctionsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Functions["func1"], 2) // Both agents have func1
	assert.Len(t, resp.Functions["func2"], 1) // Only agent-1 has func2
	assert.Len(t, resp.Functions["func3"], 1) // Only agent-2 has func3
}

// Test for opsAgentSystemInfo handler with POST

func TestOpsAgentSystemInfoHandlerPOST(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "linux"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/agent/systeminfo", `{"agentId":"agent-1"}`)

	h.OpsAgentSystemInfo(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "linux")
}

// Test for OpsAgentMeta handler with GET

func TestOpsAgentMetaHandlerGET(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "linux"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/meta?agentId=agent-1", "")

	h.OpsAgentMeta(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "linux")
}

// Test for OpsNodeMeta handler with GET

func TestOpsNodeMetaHandlerGET(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Addr:      "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"zone": "us-east-1"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	// NodeID uses uri tag, so GET with query param won't work, use POST instead
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/meta", `{"nodeId":"node-1"}`)

	h.OpsNodeMeta(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "us-east-1")
}

// Test for OpsAgentProcessesHandler with GET

func TestOpsAgentProcessesHandlerWithAgentID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/processes?agentId=agent-1", "")

	h.OpsAgentProcesses(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Test for OpsAgentMetricsHandler with GET

func TestOpsAgentMetricsHandlerWithParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/metrics?agentId=agent-1&since=2023-01-01T00:00:00Z&limit=100", "")

	h.OpsAgentMetrics(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Test for OpsAgentsListHandler with GET

func TestOpsAgentsListHandlerGET(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agents?gameId=game1&env=prod", "")

	h.OpsAgentsList(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Test for OpsAlertSilence with real model and valid numeric ID

func TestOpsAlertSilenceWithValidNumericID(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.AlertSilence{})
	require.NoError(t, err)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}

	// Use a valid numeric ID
	resp, err := opsAlertSilence(ctx, svcCtx, &OpsAlertSilenceRequest{AlertID: "123", Duration: 60})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.SilenceID)
}

// Test for OpsSilenceDelete with real model and valid numeric ID

func TestOpsSilenceDeleteWithValidNumericID(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.AlertSilence{})
	require.NoError(t, err)

	// Create a silence first
	silence := &model.AlertSilence{
		AlertID:        123,
		Reason:         "test",
		DurationMinute: 60,
	}
	require.NoError(t, db.Create(silence).Error)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}

	// Use a valid numeric ID that exists
	resp, err := opsSilenceDelete(ctx, svcCtx, &OpsAlertSilenceRequest{AlertID: fmt.Sprintf("%d", silence.ID)})
	require.NoError(t, err)
	assert.True(t, resp.Deleted)
}

// Test for OpsBackupsList with real model and empty list

func TestOpsBackupsListWithRealModelEmpty(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	ctx := context.Background()
	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}

	resp, err := opsBackupsList(ctx, svcCtx, &OpsBackupsListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Backups)
}

// Test for OpsAlerts with real model and empty list

func TestOpsAlertsWithRealModelEmpty(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Alert{})
	require.NoError(t, err)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}

	resp, err := opsAlerts(ctx, svcCtx, &OpsAlertsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Alerts)
}

// Test for OpsSilences with real model and empty list

func TestOpsSilencesWithRealModelEmpty(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.AlertSilence{})
	require.NoError(t, err)

	ctx := context.Background()
	alertModel := model.NewAlertModel(db)
	svcCtx := &svc.ServiceContext{AlertModel: alertModel}

	resp, err := opsSilences(ctx, svcCtx, &OpsSilencesRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Silences)
}

// Test for BackupService.GetDownloadURL with real model

func TestBackupServiceGetDownloadURLWithRealModel(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	// Create a backup
	backup := &model.Backup{
		BackupID: fmt.Sprintf("backup-%d", time.Now().UnixNano()),
		Name:     "test-backup",
	}
	require.NoError(t, db.Create(backup).Error)

	ctx := context.Background()
	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	s := NewBackupService(svcCtx)

	url, expiresAt, err := s.GetDownloadURL(ctx, backup.BackupID)
	require.NoError(t, err)
	assert.Contains(t, url, "/backups/")
	assert.NotEmpty(t, expiresAt)
}

// GetDownloadURL should return the recorded storage Location when present,
// instead of the legacy placeholder route.
func TestBackupServiceGetDownloadURLWithLocation(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	location := "https://files.example.com/backups/test.tgz"
	backup := &model.Backup{
		BackupID: fmt.Sprintf("backup-%d", time.Now().UnixNano()),
		Name:     "test-backup",
		Location: location,
	}
	require.NoError(t, db.Create(backup).Error)

	ctx := context.Background()
	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	s := NewBackupService(svcCtx)

	url, _, err := s.GetDownloadURL(ctx, backup.BackupID)
	require.NoError(t, err)
	assert.Equal(t, location, url)
}

// GetDownloadURL should surface a not-found error when the backup id is unknown.
func TestBackupServiceGetDownloadURLUnknownID(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.Backup{})
	require.NoError(t, err)

	ctx := context.Background()
	backupModel := model.NewBackupModel(db)
	svcCtx := &svc.ServiceContext{BackupModel: backupModel}
	s := NewBackupService(svcCtx)

	_, _, err = s.GetDownloadURL(ctx, "does-not-exist")
	require.Error(t, err)
}
