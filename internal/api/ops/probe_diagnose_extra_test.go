package ops

// probe_diagnose.go 补测：Service.ProbeDiagnose 的 422/404/未配置面与真实
// ServerProbeManager 的一次即时探测投影，以及 handler 的坏载荷 400 与往返。
// 探针目标用 127.0.0.1:1（拒绝连接，即时失败，FailThreshold=1 即开窗）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func diagnoseService(t *testing.T) *Service {
	t.Helper()
	mgr, err := svc.NewServerProbeManager([]healthprobe.TargetConfig{{
		Name: "dead-tcp", Category: healthprobe.TargetCategoryGame,
		Type: healthprobe.TargetTypeTCP, Address: "127.0.0.1:1",
		FailThreshold: 1, Interval: time.Minute, Timeout: 200 * time.Millisecond,
	}}, nil, 0, nil)
	require.NoError(t, err)
	require.NotNil(t, mgr)
	return &Service{svcCtx: &svc.ServiceContext{ServerProbes: mgr}}
}

func TestProbeDiagnoseServiceBranches(t *testing.T) {
	s := diagnoseService(t)

	// nil / 空目标 → 422
	_, err := s.ProbeDiagnose(context.Background(), nil)
	require.ErrorContains(t, err, "target 不能为空")
	_, err = s.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "  "})
	require.ErrorContains(t, err, "target 不能为空")

	// 未知目标 → 404
	_, err = s.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "nope"})
	require.Error(t, err)
	require.Equal(t, http.StatusNotFound, httpCode(err))

	// 未配置探测面 → 404（不同 message）
	bare := &Service{svcCtx: &svc.ServiceContext{}}
	_, err = bare.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "dead-tcp"})
	require.ErrorContains(t, err, "server 探测面未配置")

	// 真实即时探测 → 投影（失败目标）
	resp, err := s.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "dead-tcp"})
	require.NoError(t, err)
	require.Equal(t, "dead-tcp", resp.Target)
	require.False(t, resp.OK)
	require.False(t, resp.Available)
	require.NotEmpty(t, resp.Reason)
}

func TestProbeDiagnoseHandlerRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(diagnoseService(t))

	mkCtx := func(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(method, target, nil)
		if body != "" {
			req = httptest.NewRequest(method, target, strings.NewReader(body))
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		return c, w
	}

	// 坏 JSON → 400
	c, w := mkCtx(http.MethodPost, "/ops/probe/diagnose", "{bad")
	h.ProbeDiagnoseHandler(c)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// 空目标 → 422
	c, w = mkCtx(http.MethodPost, "/ops/probe/diagnose", `{"target":""}`)
	h.ProbeDiagnoseHandler(c)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	// 未知目标 → 404
	c, w = mkCtx(http.MethodPost, "/ops/probe/diagnose", `{"target":"nope"}`)
	h.ProbeDiagnoseHandler(c)
	require.Equal(t, http.StatusNotFound, w.Code)

	// 合法目标 → 200 + 投影
	c, w = mkCtx(http.MethodPost, "/ops/probe/diagnose", `{"target":"dead-tcp"}`)
	h.ProbeDiagnoseHandler(c)
	require.Equal(t, http.StatusOK, w.Code)
	var got ProbeDiagnoseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "dead-tcp", got.Target)
}
