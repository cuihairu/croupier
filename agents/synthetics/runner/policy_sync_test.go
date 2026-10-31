package runner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/core/register"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/require"
)

type policyUpstream struct {
	mu        sync.Mutex
	versions  map[string]string
	calls     int
	failNext  bool
	pulledIDs []string
}

func (p *policyUpstream) Connected() bool { return true }
func (p *policyUpstream) Close() error    { return nil }
func (p *policyUpstream) Register(context.Context, *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	panic("unused")
}
func (p *policyUpstream) Heartbeat(context.Context, *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	panic("unused")
}
func (p *policyUpstream) SendProbeEvent(context.Context, *opsv1.AlertEvent) error {
	return nil
}
func (p *policyUpstream) PullSyntheticsTargets(context.Context, *opsv1.SyntheticsTargetsConfigRequest) (*opsv1.SyntheticsTargetsConfigResponse, error) {
	return &opsv1.SyntheticsTargetsConfigResponse{}, nil
}
func (p *policyUpstream) PullProbePolicy(_ context.Context, req *opsv1.ProbePolicyConfigRequest) (*opsv1.ProbePolicyConfigResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.pulledIDs = append(p.pulledIDs, req.GetAgentId())
	if p.failNext {
		p.failNext = false
		return nil, errors.New("control conn reset")
	}
	version := p.versions["version"]
	if version == "" {
		return &opsv1.ProbePolicyConfigResponse{}, nil
	}
	return &opsv1.ProbePolicyConfigResponse{Version: version, PolicyJson: p.versions["policy"]}, nil
}

func newPolicyTestRunner(t *testing.T, upstream *policyUpstream) *Runner {
	t.Helper()
	r := New(config.Config{
		Agent:      config.Agent{ID: "syn-policy", GameID: "demo", Env: "prod"},
		Synthetics: config.Synthetics{Interval: time.Minute},
	}, nil)
	r.reg = register.New(register.Config{
		AgentID: r.cfg.Agent.ID,
		Dial: func(context.Context) (register.Conn, error) {
			return upstream, nil
		},
	})
	r.reg.SetConn(upstream)
	return r
}

func TestPolicySync_AppliesVersionedPolicy(t *testing.T) {
	upstream := &policyUpstream{versions: map[string]string{
		"version": "v1",
		"policy":  `{"interval":"7s","targets":[{"target":"web","interval":"9s"}]}`,
	}}
	r := newPolicyTestRunner(t, upstream)
	puller := r.newPolicyPuller()

	applied, err := puller.PullOnce(context.Background())
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, "v1", puller.LastVersion())
	require.NotNil(t, r.policy.Load())
	require.Equal(t, 9*time.Second, r.intervalFor("web", false))
	require.Equal(t, 7*time.Second, r.intervalFor("other", false))

	require.Equal(t, "syn-policy", upstream.pulledIDs[0])
}

func TestPolicySync_FetchFailureKeepsLastGood(t *testing.T) {
	upstream := &policyUpstream{versions: map[string]string{
		"version": "v1",
		"policy":  `{"interval":"7s"}`,
	}}
	r := newPolicyTestRunner(t, upstream)
	puller := r.newPolicyPuller()
	_, err := puller.PullOnce(context.Background())
	require.NoError(t, err)

	upstream.failNext = true
	applied, err := puller.PullOnce(context.Background())
	require.Error(t, err)
	require.False(t, applied)
	require.Equal(t, "v1", puller.LastVersion())
	require.NotNil(t, r.policy.Load())
	require.Equal(t, 7*time.Second, r.intervalFor("target", false))
}

func TestPolicySync_InvalidPayloadKeepsLastGood(t *testing.T) {
	upstream := &policyUpstream{versions: map[string]string{
		"version": "v1",
		"policy":  `{"interval":"7s"}`,
	}}
	r := newPolicyTestRunner(t, upstream)
	puller := r.newPolicyPuller()
	_, err := puller.PullOnce(context.Background())
	require.NoError(t, err)

	upstream.versions["version"] = "v2"
	upstream.versions["policy"] = `not-json`
	applied, err := puller.PullOnce(context.Background())
	require.Error(t, err)
	require.False(t, applied)
	require.Equal(t, "v1", puller.LastVersion())
	require.Equal(t, 7*time.Second, r.intervalFor("target", false))
}
