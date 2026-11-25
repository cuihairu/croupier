package ops

// Manual diagnostic probe endpoint tests: probe surface not configured → 404,
// empty target → 422, unknown target → 404, and a hit returns the single-result
// projection.

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

func newProbeDiagnoseService(mgr *svc.ServerProbeManager) *Service {
	return &Service{svcCtx: &svc.ServiceContext{ServerProbes: mgr}}
}

func diagnoseTestManager(t *testing.T) *svc.ServerProbeManager {
	t.Helper()
	m, err := svc.NewServerProbeManager([]healthprobe.TargetConfig{
		{Name: "dead-tcp", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: "127.0.0.1:1",
			FailThreshold: 1, Interval: time.Minute},
	}, nil, 0, nil)
	require.NoError(t, err)
	require.NotNil(t, m)
	return m
}

func errorHTTPStatus(t *testing.T, err error) int {
	t.Helper()
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "error must be *errorx.CodeError, got %T", err)
	return codeErr.Code
}

func TestProbeDiagnose_SurfaceNotConfigured(t *testing.T) {
	s := newProbeDiagnoseService(nil)
	_, err := s.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "any"})
	require.Error(t, err)
	assert.Equal(t, http.StatusNotFound, errorHTTPStatus(t, err))
}

func TestProbeDiagnose_TargetRequired(t *testing.T) {
	s := newProbeDiagnoseService(diagnoseTestManager(t))
	_, err := s.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "  "})
	require.Error(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, errorHTTPStatus(t, err))
}

func TestProbeDiagnose_UnknownTarget(t *testing.T) {
	s := newProbeDiagnoseService(diagnoseTestManager(t))
	_, err := s.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "nope"})
	require.Error(t, err)
	assert.Equal(t, http.StatusNotFound, errorHTTPStatus(t, err))
}

func TestProbeDiagnose_ReturnsResult(t *testing.T) {
	s := newProbeDiagnoseService(diagnoseTestManager(t))
	resp, err := s.ProbeDiagnose(context.Background(), &ProbeDiagnoseRequest{Target: "dead-tcp"})
	require.NoError(t, err)
	assert.Equal(t, "dead-tcp", resp.Target)
	assert.False(t, resp.OK)
	assert.False(t, resp.Available)
	assert.NotEmpty(t, resp.Reason)
	// failThreshold=1: a single failure opens the window (manual diagnosis and
	// periodic probing share the state machine).
	assert.True(t, resp.WindowOpen)
}
