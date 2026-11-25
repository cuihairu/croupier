package ops

// Extra coverage for probe_diagnose.go: the 422/404/not-configured surface of
// Service.ProbeDiagnose with a real ServerProbeManager's single instant-probe
// projection, plus the handler's bad-payload 400 and round trip. The probe
// target is 127.0.0.1:1 (connection refused, instant failure, FailThreshold=1
// opens the window immediately).

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

	// nil / empty target → 422
	_, err := s.ProbeDiagnose(context.Background(), nil)
	require.ErrorContains(t, err, "target 不能为空")
	_, err = s.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "  "})
	require.ErrorContains(t, err, "target 不能为空")

	// unknown target → 404
	_, err = s.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "nope"})
	require.Error(t, err)
	require.Equal(t, http.StatusNotFound, httpCode(err))

	// probe surface not configured → 404 (different message)
	bare := &Service{svcCtx: &svc.ServiceContext{}}
	_, err = bare.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "dead-tcp"})
	require.ErrorContains(t, err, "server 探测面未配置")

	// real instant probe → projection (failing target)
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

	// bad JSON → 400
	c, w := mkCtx(http.MethodPost, "/ops/probe/diagnose", "{bad")
	h.ProbeDiagnoseHandler(c)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// empty target → 422
	c, w = mkCtx(http.MethodPost, "/ops/probe/diagnose", `{"target":""}`)
	h.ProbeDiagnoseHandler(c)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	// unknown target → 404
	c, w = mkCtx(http.MethodPost, "/ops/probe/diagnose", `{"target":"nope"}`)
	h.ProbeDiagnoseHandler(c)
	require.Equal(t, http.StatusNotFound, w.Code)

	// valid target → 200 + projection
	c, w = mkCtx(http.MethodPost, "/ops/probe/diagnose", `{"target":"dead-tcp"}`)
	h.ProbeDiagnoseHandler(c)
	require.Equal(t, http.StatusOK, w.Code)
	var got ProbeDiagnoseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "dead-tcp", got.Target)
}
