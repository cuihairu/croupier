package monitoring

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestRouter 按生产形态组装 engine：开关决定是否挂载端点，
// AuthMiddleware 决定是否免认证。
func buildTestRouter(t *testing.T, promEnabled bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Config{}
	cfg.Telemetry.Prometheus.Enabled = promEnabled

	serverCtx := &svc.ServiceContext{Config: cfg}
	r := gin.New()
	r.Use(svc.NewAuthMiddleware(serverCtx))
	if promEnabled {
		r.GET(cfg.Telemetry.Prometheus.PrometheusPath(), gin.WrapH(NewPrometheusHandler(serverCtx)))
	}
	return r
}

func TestPrometheusEndpoint_DisabledByDefault(t *testing.T) {
	r := buildTestRouter(t, false)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metrics/prometheus", nil))
	assert.NotEqual(t, 200, w.Code, "disabled endpoint must not be reachable")
}

func TestPrometheusEndpoint_EnabledReturnsExposition(t *testing.T) {
	r := buildTestRouter(t, true)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics/prometheus", nil)
	req = req.WithContext(context.Background())
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, "enabled endpoint must be reachable without auth")

	body := w.Body.String()
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	// Go runtime 指标 + 平台指标同场暴露
	assert.Contains(t, body, "go_goroutines")
	assert.Contains(t, body, "croupier_db_up")
	assert.Contains(t, body, "croupier_agents_total")
	assert.Contains(t, body, "croupier_functions_registered")
}

func TestPrometheusEndpoint_CustomPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{}
	cfg.Telemetry.Prometheus.Enabled = true
	cfg.Telemetry.Prometheus.Path = "/ops/prom"

	serverCtx := &svc.ServiceContext{Config: cfg}
	r := gin.New()
	r.Use(svc.NewAuthMiddleware(serverCtx))
	r.GET(cfg.Telemetry.Prometheus.PrometheusPath(), gin.WrapH(NewPrometheusHandler(serverCtx)))

	// 自定义路径可达（无 Authorization 头仍 200，证明白名单生效）
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ops/prom", nil))
	assert.Equal(t, 200, w.Code)

	// 默认路径不存在
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/metrics/prometheus", nil))
	assert.NotEqual(t, 200, w2.Code)
}
