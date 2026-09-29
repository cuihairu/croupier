package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPerformanceRouter(t *testing.T) (*gin.Engine, *svc.ServiceContext) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svcCtx := &svc.ServiceContext{
		PlatformSettingModel: newPerfStore(t),
	}
	r := gin.New()
	h := NewHandler(NewService(svcCtx))
	r.GET("/ops/performance", h.PerformanceGet)
	r.PUT("/ops/performance", func(c *gin.Context) {
		c.Set("username", "tester")
		h.PerformancePut(c)
	})
	return r, svcCtx
}

// newPerfStore 内存库 settings model（同 sitesettings 测试口径：sqlite 建表）。
func newPerfStore(t *testing.T) *model.PlatformSettingModel {
	t.Helper()
	store := model.NewPlatformSettingModel(newPerfDB(t))
	return store
}

func newPerfDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/perf.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

// initPerfSettings 重置全局 layered 并指向独立内存库（避免串包状态泄漏）。
func initPerfSettings(t *testing.T) {
	t.Helper()
	settings.ResetForTest()
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, newPerfStore(t))
}

func TestPerformanceGet_SnapshotShape(t *testing.T) {
	initPerfSettings(t)
	r, _ := setupPerformanceRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ops/performance", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var body PerformanceSnapshotResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	// 默认全 0 = 不启用
	assert.Equal(t, 0, body.Settings.MaxCpuPct)
	assert.False(t, body.Overload.CPU)
	// 运行时统计恒有值
	assert.Greater(t, body.Runtime.GoMaxProcs, 0)
	assert.GreaterOrEqual(t, body.Runtime.Goroutines, 1)
	assert.GreaterOrEqual(t, body.Runtime.UptimeSeconds, int64(0))
	assert.False(t, body.Settings.Sources == nil)
}

func TestPerformancePut_WriteAndSource(t *testing.T) {
	initPerfSettings(t)
	r, _ := setupPerformanceRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/ops/performance",
		strings.NewReader(`{"perf.maxCpuPct":80,"perf.maxConcurrent":500}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body PerformanceSnapshotResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 80, body.Settings.MaxCpuPct)
	assert.Equal(t, 500, body.Settings.MaxConcurrent)
	assert.Equal(t, "database", body.Settings.Sources["maxCpuPct"])
	assert.Equal(t, "default", body.Settings.Sources["maxThreadCount"])
}

func TestPerformancePut_Rejections(t *testing.T) {
	initPerfSettings(t)
	r, _ := setupPerformanceRouter(t)

	cases := []struct {
		name string
		body map[string]int64
		code int
	}{
		{"未知键", map[string]int64{"perf.hack": 1}, http.StatusBadRequest},
		{"负数", map[string]int64{"perf.maxCpuPct": -1}, http.StatusBadRequest},
		{"百分比越界", map[string]int64{"perf.maxMemoryPct": 150}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		raw, _ := json.Marshal(tc.body)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/ops/performance", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, tc.code, w.Code, tc.name)
	}
}

func TestPerformancePut_NilStoreFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(nil))
	r.PUT("/ops/performance", h.PerformancePut)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/ops/performance",
		strings.NewReader(`{"perf.maxCpuPct":50}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestBuildPerformanceSnapshot_OverloadThresholds(t *testing.T) {
	initPerfSettings(t)

	snap := buildPerformanceSnapshot(nil)
	assert.False(t, snap.Overload.CPU, "阈值 0 = 不启用")

	// 阈值 101% 永不触发（占用不可能 >= 101）
	snap.Settings.MaxCpuPct = 101
	snap.Overload = PerformanceOverload{
		CPU: snap.Settings.MaxCpuPct > 0 && snap.Host.CPUPercent >= float64(snap.Settings.MaxCpuPct),
	}
	assert.False(t, snap.Overload.CPU)
}
