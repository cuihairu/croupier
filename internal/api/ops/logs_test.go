package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogsRouter(t *testing.T, mutate func(*svc.ServiceContext)) (*gin.Engine, *svc.ServiceContext) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/logs.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	svcCtx := &svc.ServiceContext{
		PlatformSettingModel: model.NewPlatformSettingModel(db),
		DB:                   db,
		Config:               config.Config{},
	}
	if mutate != nil {
		mutate(svcCtx)
	}
	r := gin.New()
	h := NewHandler(NewService(svcCtx))
	r.GET("/ops/logs", h.LogsGet)
	r.PUT("/ops/logs", func(c *gin.Context) {
		c.Set("username", "tester")
		h.LogsPut(c)
	})
	r.POST("/ops/logs/cleanup", h.LogsCleanupPost)
	return r, svcCtx
}

func initLogsSettings(t *testing.T) {
	t.Helper()
	settings.ResetForTest()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/logs_settings.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, model.NewPlatformSettingModel(db))
}

func seedLogsFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -30)
	recent := now.Add(-time.Hour)
	logs := []model.ExecutionLog{
		{GameID: "g1", Env: "prod", Source: "invoke", FunctionID: "f.old", Actor: "alice", Status: "ok", CreatedAt: old},
		{GameID: "g1", Env: "prod", Source: "invoke", FunctionID: "f.new", Actor: "alice", Status: "ok", CreatedAt: recent},
	}
	require.NoError(t, db.Create(&logs).Error)
	runs := []model.TaskRun{
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-old", FunctionID: "job.old", Status: "success"},
		{Model: gorm.Model{CreatedAt: recent, UpdatedAt: recent}, TaskID: "t-new", FunctionID: "job.new", Status: "success"},
	}
	require.NoError(t, db.Create(&runs).Error)
}

func TestLogsGet_SnapshotShape(t *testing.T) {
	initLogsSettings(t)
	r, svcCtx := setupLogsRouter(t, nil)
	seedLogsFixtures(t, svcCtx.DB)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ops/logs", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var body LogsSnapshotResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	// L3 默认 0 → effective 走配置文件缺省（各 7 天）
	assert.Equal(t, 0, body.Settings.RetentionDays)
	assert.Equal(t, 7, body.Effective.ExecutionLogDays)
	assert.Equal(t, 7, body.Effective.TaskLogDays)
	// 服务器日志视图：未配置 file 输出 → stdout
	assert.Equal(t, "stdout", body.ServerLog.Output)
	// 三张留痕表体量
	require.Len(t, body.Tables, 3)
	byName := map[string]LogTableView{}
	for _, tv := range body.Tables {
		byName[tv.Table] = tv
	}
	assert.Equal(t, int64(2), byName["execution_logs"].Rows)
	assert.Equal(t, int64(2), byName["task_runs"].Rows)
	assert.Equal(t, int64(0), byName["task_events"].Rows)
	assert.NotEmpty(t, byName["execution_logs"].OldestAt)
}

func TestLogsGet_L3OverrideEffective(t *testing.T) {
	initLogsSettings(t)
	r, svcCtx := setupLogsRouter(t, nil)
	require.NoError(t, svcCtx.PlatformSettingModel.Set(context.Background(),
		settings.KeyLogRetentionDays, json.RawMessage(`30`), "tester"))
	settings.Current().Reload(context.Background(), svcCtx.PlatformSettingModel)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ops/logs", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var body LogsSnapshotResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 30, body.Settings.RetentionDays)
	assert.Equal(t, "database", body.Settings.Sources["retentionDays"])
	assert.Equal(t, 30, body.Effective.ExecutionLogDays)
	assert.Equal(t, 30, body.Effective.TaskLogDays)
}

func TestLogsPut_WriteAndReject(t *testing.T) {
	initLogsSettings(t)
	r, _ := setupLogsRouter(t, nil)

	// 合法写入
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/ops/logs", strings.NewReader(`{"log.retentionDays":14}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body LogsSnapshotResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 14, body.Settings.RetentionDays)
	assert.Equal(t, "database", body.Settings.Sources["retentionDays"])

	// 拒绝：未知键 / 负数 / 越界
	cases := []struct {
		name string
		body string
		code int
	}{
		{"未知键", `{"log.copierKeep":5}`, http.StatusBadRequest},
		{"负数", `{"log.retentionDays":-1}`, http.StatusBadRequest},
		{"越界", `{"log.retentionDays":36501}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/ops/logs", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, tc.code, w.Code, tc.name)
	}
}

func TestLogsCleanup_DeletesOnlyBeforeCutoff(t *testing.T) {
	initLogsSettings(t)
	r, svcCtx := setupLogsRouter(t, nil)
	seedLogsFixtures(t, svcCtx.DB)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ops/logs/cleanup",
		strings.NewReader(`{"scope":"all","beforeHours":168}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body LogsCleanupResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, int64(1), body.ExecutionLogsDeleted)
	assert.Equal(t, int64(1), body.TaskRunsDeleted)
	assert.NotEmpty(t, body.Cutoff)

	// 各表仅剩新记录
	var logCount, runCount int64
	require.NoError(t, svcCtx.DB.Model(&model.ExecutionLog{}).Count(&logCount).Error)
	require.NoError(t, svcCtx.DB.Model(&model.TaskRun{}).Count(&runCount).Error)
	assert.Equal(t, int64(1), logCount)
	assert.Equal(t, int64(1), runCount)
}

func TestLogsCleanup_Rejections(t *testing.T) {
	initLogsSettings(t)
	r, _ := setupLogsRouter(t, nil)

	cases := []struct {
		name string
		body string
		code int
	}{
		{"非法 scope", `{"scope":"hack","beforeHours":24}`, http.StatusBadRequest},
		{"零小时", `{"scope":"all","beforeHours":0}`, http.StatusBadRequest},
		{"负小时", `{"scope":"all","beforeHours":-5}`, http.StatusBadRequest},
		{"越界", `{"scope":"all","beforeHours":87601}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ops/logs/cleanup", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, tc.code, w.Code, tc.name)
	}
}

// TestLogsCleanup_RetentionFanoutPath LogRetention 存在时走 PurgeBefore
// （multiGame fanout 同路径；单库下行为与直清一致）。
func TestLogsCleanup_RetentionFanoutPath(t *testing.T) {
	initLogsSettings(t)
	r, svcCtx := setupLogsRouter(t, func(s *svc.ServiceContext) {
		s.LogRetention = executionlog.NewRetention(s.DB, executionlog.RetentionConfig{})
	})
	seedLogsFixtures(t, svcCtx.DB)

	raw, _ := json.Marshal(LogsCleanupRequest{Scope: "execution", BeforeHours: 24})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ops/logs/cleanup", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body LogsCleanupResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, int64(1), body.ExecutionLogsDeleted)
	assert.Equal(t, int64(0), body.TaskRunsDeleted, "scope=execution 不清任务留痕")

	var runCount int64
	require.NoError(t, svcCtx.DB.Model(&model.TaskRun{}).Count(&runCount).Error)
	assert.Equal(t, int64(2), runCount, "task_runs 不受 execution scope 影响")
}

// TestBuildServerLogView_FileOutput 配置 file 输出时读目录计数（含当前文件）。
func TestBuildServerLogView_FileOutput(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"croupier.log", "croupier-2026-09-01T00-00-00.000.log", "croupier-2026-09-02T00-00-00.000.log", "other.log"} {
		require.NoError(t, os.WriteFile(dir+"/"+name, []byte("x"), 0o600))
	}
	svcCtx := &svc.ServiceContext{Config: config.Config{}}
	svcCtx.Config.Logging.Output = "file"
	svcCtx.Config.Logging.File = dir + "/croupier.log"
	svcCtx.Config.Logging.MaxSize = 100
	svcCtx.Config.Logging.MaxBackups = 5

	view := buildServerLogView(svcCtx)
	assert.Equal(t, "file", view.Output)
	assert.Equal(t, dir, view.Directory)
	assert.Equal(t, 100, view.MaxSizeMB)
	assert.Equal(t, 5, view.MaxBackups)
	assert.Equal(t, 3, view.FileCount, "当前文件 + 两个轮转备份，other.log 不计")
}

// TestBuildLogTableViews_NilDB 库不可达时 Rows=-1 降级，不 panic。
func TestBuildLogTableViews_NilDB(t *testing.T) {
	views := buildLogTableViews(&svc.ServiceContext{})
	require.Len(t, views, 3)
	for _, v := range views {
		assert.Equal(t, int64(-1), v.Rows)
		assert.Empty(t, v.OldestAt)
	}
}
