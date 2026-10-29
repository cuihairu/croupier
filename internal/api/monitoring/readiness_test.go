package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
)

// ARCH-004 就绪探针：DB 不可用 → 不就绪（带原因）；集群要求且依赖不可用
// → 不就绪，除非显式 allowStandaloneDegradation（降级为 degraded=true）。

func TestReadiness_DatabaseUnavailable_NotReady(t *testing.T) {
	svcCtx := &svc.ServiceContext{} // DB nil
	svc := NewService(svcCtx)
	resp, err := svc.Readiness(context.Background(), &ReadinessRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Ready)
	assert.False(t, resp.Degraded)
	require.Len(t, resp.Reasons, 1)
	assert.Contains(t, resp.Reasons[0], "database")
}

func TestReadiness_ClusterDegradedNoFlag_NotReady(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{},
	}
	svcCtx.ClusterReady = func() (bool, []string) {
		return false, []string{"cluster: lease renew failing: redis: connection refused"}
	}
	svc := NewService(svcCtx)
	resp, err := svc.Readiness(context.Background(), &ReadinessRequest{})
	require.NoError(t, err)
	// DB 也不可用（nil DB），故 reasons 至少两条：database + cluster
	assert.False(t, resp.Ready)
	assert.False(t, resp.Degraded)
	assert.True(t, containsReason(resp.Reasons, "cluster: lease renew failing"))
	assert.True(t, containsReason(resp.Reasons, "database"))
}

func TestReadiness_ClusterDegradedWithFlag_Degraded(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{
		DB:     db,
		Config: config.Config{},
	}
	svcCtx.Config.Cluster.AllowStandaloneDegradation = true
	svcCtx.ClusterReady = func() (bool, []string) {
		return false, []string{"cluster: member registration failed"}
	}
	svc := NewService(svcCtx)
	resp, err := svc.Readiness(context.Background(), &ReadinessRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Ready)    // 显式降级允许：仍就绪
	assert.True(t, resp.Degraded) // 但标记降级
	assert.Empty(t, resp.Reasons)
}

func TestReadiness_ClusterHealthy_Ready(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{
		DB:     db,
		Config: config.Config{},
	}
	svcCtx.ClusterReady = func() (bool, []string) { return true, nil }
	svc := NewService(svcCtx)
	resp, err := svc.Readiness(context.Background(), &ReadinessRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Ready)
	assert.False(t, resp.Degraded)
	assert.Empty(t, resp.Reasons)
}

func TestReadiness_ClusterNotRequired_SkipsClusterCheck(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// ClusterReady nil = 集群未要求
	svcCtx := &svc.ServiceContext{DB: db, Config: config.Config{}}
	svc := NewService(svcCtx)
	resp, err := svc.Readiness(context.Background(), &ReadinessRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Ready)
	assert.False(t, resp.Degraded)
}

func TestReadiness_HandlerMapsStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 不健康 → 503
	svcCtxNotReady := &svc.ServiceContext{}
	routerNotReady := gin.New()
	routerNotReady.GET("/readyz", NewHandler(NewService(svcCtxNotReady)).Readiness)
	w := httptest.NewRecorder()
	routerNotReady.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	// 健康 → 200
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	svcCtxReady := &svc.ServiceContext{DB: db, Config: config.Config{}}
	routerReady := gin.New()
	routerReady.GET("/readyz", NewHandler(NewService(svcCtxReady)).Readiness)
	w2 := httptest.NewRecorder()
	routerReady.ServeHTTP(w2, httptest.NewRequest("GET", "/readyz", nil))
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), `"ready":true`)
}

func containsReason(reasons []string, sub string) bool {
	for _, r := range reasons {
		if len(r) >= len(sub) && r[:len(sub)] == sub {
			return true
		}
	}
	return false
}
