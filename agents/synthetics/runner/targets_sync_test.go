package runner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/core/register"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/require"
)

type targetsUpstream struct {
	mu        sync.Mutex
	versions  map[string]string
	failNext  bool
	calls     int
	pulledIDs []string
}

func (p *targetsUpstream) Connected() bool { return true }
func (p *targetsUpstream) Close() error    { return nil }
func (p *targetsUpstream) Register(context.Context, *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	panic("unused")
}
func (p *targetsUpstream) Heartbeat(context.Context, *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	panic("unused")
}
func (p *targetsUpstream) SendProbeEvent(context.Context, *opsv1.AlertEvent) error {
	return nil
}
func (p *targetsUpstream) PullProbePolicy(context.Context, *opsv1.ProbePolicyConfigRequest) (*opsv1.ProbePolicyConfigResponse, error) {
	return &opsv1.ProbePolicyConfigResponse{}, nil
}
func (p *targetsUpstream) PullSyntheticsTargets(_ context.Context, req *opsv1.SyntheticsTargetsConfigRequest) (*opsv1.SyntheticsTargetsConfigResponse, error) {
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
		return &opsv1.SyntheticsTargetsConfigResponse{}, nil
	}
	return &opsv1.SyntheticsTargetsConfigResponse{Version: version, TargetsJson: p.versions["targets"]}, nil
}

func newTargetsTestRunner(t *testing.T, upstream *targetsUpstream) *Runner {
	t.Helper()
	r := New(config.Config{
		Agent: config.Agent{ID: "syn-targets", GameID: "demo", Env: "prod"},
		Synthetics: config.Synthetics{Interval: time.Minute, Targets: []config.Target{{
			Name: "local", URL: "http://127.0.0.1", Method: "GET",
			Timeout: time.Second, ExpectedStatus: 200, TargetCategory: "game",
		}}},
	}, nil)
	r.reg = register.New(register.Config{
		AgentID: r.cfg.Agent.ID,
		Dial: func(context.Context) (register.Conn, error) {
			return upstream, nil
		},
	})
	r.reg.SetConn(upstream)
	r.targets = newTargetSet()
	r.wake = make(chan struct{}, 1)
	r.addTarget(r.cfg.Synthetics.Targets[0], false)
	return r
}

func TestTargetsSync_AppliesVersionedTargets(t *testing.T) {
	upstream := &targetsUpstream{versions: map[string]string{
		"version": "v1",
		"targets": `[{"name":"web","url":"http://127.0.0.1","method":"GET","timeout":"2s","expectedStatus":200,"targetCategory":"game"}]`,
	}}
	r := newTargetsTestRunner(t, upstream)
	puller := r.newTargetsPuller()

	applied, err := puller.PullOnce(context.Background())
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, "v1", puller.LastVersion())
	require.Equal(t, "syn-targets", upstream.pulledIDs[0])

	// local target replaced by the pulled set.
	require.Nil(t, r.targets.get("local"))
	require.NotNil(t, r.targets.get("web"))
}

func TestTargetsSync_SameVersionNoOp(t *testing.T) {
	upstream := &targetsUpstream{versions: map[string]string{
		"version": "v1",
		"targets": `[{"name":"web","url":"http://127.0.0.1","method":"GET","timeout":"2s","expectedStatus":200,"targetCategory":"game"}]`,
	}}
	r := newTargetsTestRunner(t, upstream)
	puller := r.newTargetsPuller()
	_, err := puller.PullOnce(context.Background())
	require.NoError(t, err)

	applied, err := puller.PullOnce(context.Background())
	require.NoError(t, err)
	require.False(t, applied)
	require.Equal(t, "v1", puller.LastVersion())
}

func TestTargetsSync_FetchFailureKeepsLastGood(t *testing.T) {
	upstream := &targetsUpstream{versions: map[string]string{
		"version": "v1",
		"targets": `[{"name":"web","url":"http://127.0.0.1","method":"GET","timeout":"2s","expectedStatus":200,"targetCategory":"game"}]`,
	}}
	r := newTargetsTestRunner(t, upstream)
	puller := r.newTargetsPuller()
	_, err := puller.PullOnce(context.Background())
	require.NoError(t, err)

	upstream.failNext = true
	applied, err := puller.PullOnce(context.Background())
	require.Error(t, err)
	require.False(t, applied)
	require.Equal(t, "v1", puller.LastVersion())
	require.NotNil(t, r.targets.get("web"))
}

func TestTargetsSync_InvalidPayloadKeepsLastGood(t *testing.T) {
	upstream := &targetsUpstream{versions: map[string]string{
		"version": "v1",
		"targets": `[{"name":"web","url":"http://127.0.0.1","method":"GET","timeout":"2s","expectedStatus":200,"targetCategory":"game"}]`,
	}}
	r := newTargetsTestRunner(t, upstream)
	puller := r.newTargetsPuller()
	_, err := puller.PullOnce(context.Background())
	require.NoError(t, err)

	upstream.versions["version"] = "v2"
	upstream.versions["targets"] = `not-json`
	applied, err := puller.PullOnce(context.Background())
	require.Error(t, err)
	require.False(t, applied)
	require.Equal(t, "v1", puller.LastVersion())
	require.NotNil(t, r.targets.get("web"))
}

func TestTargetsSync_InvalidTargetKeepsLastGood(t *testing.T) {
	upstream := &targetsUpstream{versions: map[string]string{
		"version": "v1",
		"targets": `[{"name":"web","url":"http://127.0.0.1","method":"GET","timeout":"2s","expectedStatus":200,"targetCategory":"game"}]`,
	}}
	r := newTargetsTestRunner(t, upstream)
	puller := r.newTargetsPuller()
	_, err := puller.PullOnce(context.Background())
	require.NoError(t, err)

	upstream.versions["version"] = "v2"
	upstream.versions["targets"] = `[{"name":"web","url":"http://127.0.0.1","method":"GET","timeout":"2s","expectedStatus":200,"targetCategory":"bogus"}]`
	applied, err := puller.PullOnce(context.Background())
	require.Error(t, err)
	require.False(t, applied)
	require.Equal(t, "v1", puller.LastVersion())
	require.NotNil(t, r.targets.get("web"))
}

func TestTargetsSync_RemovalDisablesTarget(t *testing.T) {
	upstream := &targetsUpstream{versions: map[string]string{
		"version": "v1",
		"targets": `[{"name":"web","url":"http://127.0.0.1","method":"GET","timeout":"2s","expectedStatus":200,"targetCategory":"game"}]`,
	}}
	r := newTargetsTestRunner(t, upstream)
	puller := r.newTargetsPuller()
	_, err := puller.PullOnce(context.Background())
	require.NoError(t, err)
	require.NotNil(t, r.targets.get("web"))

	upstream.versions["version"] = "v2"
	upstream.versions["targets"] = `[]`
	applied, err := puller.PullOnce(context.Background())
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, "v2", puller.LastVersion())
	require.Nil(t, r.targets.get("web"))
	require.Empty(t, r.targets.snapshot())
}

func TestTargetsSync_PolicyIntervalAppliesToNewTarget(t *testing.T) {
	upstream := &targetsUpstream{versions: map[string]string{
		"version": "v1",
		"targets": `[{"name":"web","url":"http://127.0.0.1","method":"GET","timeout":"2s","expectedStatus":200,"targetCategory":"game"}]`,
	}}
	r := newTargetsTestRunner(t, upstream)
	// Apply a policy that gives "web" a 9s interval before the target arrives.
	r.policy.Store(mustParsePolicy(t, `{"interval":"7s","autoBoost":false,"targets":[{"target":"web","interval":"9s"}]}`))

	puller := r.newTargetsPuller()
	_, err := puller.PullOnce(context.Background())
	require.NoError(t, err)
	loop := r.targets.get("web")
	require.NotNil(t, loop)
	require.Equal(t, 9*time.Second, loop.targetInterval)
	require.Equal(t, 9*time.Second, r.intervalFor("web", false))
}

func mustParsePolicy(t *testing.T, content string) *healthprobe.ProbePolicy {
	t.Helper()
	p, err := healthprobe.ParseProbePolicy(content)
	require.NoError(t, err)
	return p
}
