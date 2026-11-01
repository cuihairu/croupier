package ops

import (
	"context"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/require"
)

func TestDeployAgentHealthyNoMetrics(t *testing.T) {
	a := &DeployAgent{ctx: &svc.ServiceContext{}}
	ok, reason := a.Healthy(context.Background(), "a")
	require.True(t, ok)
	require.Contains(t, reason, "assume healthy")
}

func TestDeployAgentHealthyProbeFail(t *testing.T) {
	store := reg.NewMetricsStore()
	store.Add("a", &opsv1.MetricsReport{
		ProbeStats: []*opsv1.ProbeStats{{Target: "web", Checked: 3, Failed: 1}},
	})
	a := &DeployAgent{ctx: &svc.ServiceContext{MetricsStore: store}}
	ok, reason := a.Healthy(context.Background(), "a")
	require.False(t, ok)
	require.Contains(t, reason, "probe failed")
}

func TestDeployAgentHealthyProbePass(t *testing.T) {
	store := reg.NewMetricsStore()
	store.Add("a", &opsv1.MetricsReport{
		ProbeStats: []*opsv1.ProbeStats{{Target: "web", Checked: 3, Failed: 0}},
	})
	a := &DeployAgent{ctx: &svc.ServiceContext{MetricsStore: store}}
	ok, _ := a.Healthy(context.Background(), "a")
	require.True(t, ok)
}
