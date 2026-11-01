package deploy

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// fakeStore 内存持久化。
type fakeStore struct {
	items map[string]*model.Deployment
}

func newFakeStore() *fakeStore { return &fakeStore{items: map[string]*model.Deployment{}} }

func (f *fakeStore) Create(_ context.Context, d *model.Deployment) error {
	f.items[d.DeploymentID] = d
	return nil
}
func (f *fakeStore) Update(_ context.Context, d *model.Deployment) error {
	f.items[d.DeploymentID] = d
	return nil
}
func (f *fakeStore) FindByID(_ context.Context, id string) (*model.Deployment, error) {
	d, ok := f.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return d, nil
}
func (f *fakeStore) List(_ context.Context, _ int) ([]model.Deployment, error) {
	var out []model.Deployment
	for _, d := range f.items {
		out = append(out, *d)
	}
	return out, nil
}

// fakeAgent 可注入投递/换包/健康行为。
type fakeAgent struct {
	stageErr   map[string]error
	applyErr   map[string]error
	healthy    map[string]bool
	applied    []string
	stageCount map[string]int
}

func newFakeAgent() *fakeAgent {
	return &fakeAgent{
		stageErr:   map[string]error{},
		applyErr:   map[string]error{},
		healthy:    map[string]bool{},
		applied:    []string{},
		stageCount: map[string]int{},
	}
}

func (f *fakeAgent) StagePackage(_ context.Context, agentID string, pkg *model.DeployPackage) (string, error) {
	f.stageCount[agentID]++
	if err, ok := f.stageErr[agentID]; ok && err != nil {
		return "", err
	}
	return "/staged/" + agentID + "/" + pkg.PackageID, nil
}
func (f *fakeAgent) ApplyUpdate(_ context.Context, agentID, _, _, fp string) (string, error) {
	if err, ok := f.applyErr[agentID]; ok && err != nil {
		return "", err
	}
	f.applied = append(f.applied, agentID)
	return "fp-" + fp[:8], nil
}
func (f *fakeAgent) Healthy(_ context.Context, agentID string) (bool, string) {
	return f.healthy[agentID], ""
}

func pkg() *model.DeployPackage {
	return &model.DeployPackage{PackageID: "pkg-1", Version: "2.0.0", ObjectKey: "deploy/p/1.bin", Checksum: "abcdef0123456789"}
}

func TestBatchPercent(t *testing.T) {
	for _, v := range []int{1, 10, 50, 100} {
		require.True(t, BatchPercent(v), "valid %d", v)
	}
	for _, v := range []int{0, 25, 99} {
		require.False(t, BatchPercent(v), "invalid %d", v)
	}
}

func TestNewDeploymentSplitsBatches(t *testing.T) {
	d, err := NewDeployment("pkg-1", "d1", "g1", "prod", "game-server", "game", 10, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, "admin")
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateDraft, d.State)
	targets := decodeTargets(d.TargetsJSON)
	require.Len(t, targets, 10)
	// 10% of 10 = 1 per batch → 10 batches.
	require.Equal(t, 10, d.BatchCount)
	require.Equal(t, 1, targets[0].Batch)
	require.Equal(t, 10, targets[9].Batch)
}

func TestStartCompletesAllBatches(t *testing.T) {
	store := newFakeStore()
	agent := newFakeAgent()
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		agent.healthy[id] = true
	}
	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1

	d, err := NewDeployment("pkg-1", "d1", "g1", "prod", "game-server", "game", 10, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, "admin")
	require.NoError(t, err)
	require.NoError(t, store.Create(context.Background(), d))

	started, err := o.Start(context.Background(), d, pkg())
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateComplete, started.State)
	require.Len(t, agent.applied, 10)
	targets := decodeTargets(started.TargetsJSON)
	for _, tg := range targets {
		require.Equal(t, model.DeploymentTargetHealthy, tg.State)
	}
}

func TestHealthGatePausesBatch(t *testing.T) {
	store := newFakeStore()
	agent := newFakeAgent()
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		agent.healthy[id] = true
	}
	// 第二批第一机 c 不健康。
	agent.healthy["c"] = false
	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1

	d, err := NewDeployment("pkg-1", "d1", "g1", "prod", "game-server", "game", 10, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, "admin")
	require.NoError(t, err)
	require.NoError(t, store.Create(context.Background(), d))

	started, err := o.Start(context.Background(), d, pkg())
	require.Error(t, err)
	require.Equal(t, model.DeploymentStatePaused, started.State)
	require.Contains(t, started.PausedReason, "health gate failed")
	// a、b 已健康，c 失败暂停，d..j 未触达。
	targets := decodeTargets(started.TargetsJSON)
	require.Equal(t, model.DeploymentTargetHealthy, targets[0].State)
	require.Equal(t, model.DeploymentTargetHealthy, targets[1].State)
	require.Equal(t, model.DeploymentTargetFailed, targets[2].State)
	require.Equal(t, model.DeploymentTargetPending, targets[3].State)
}

func TestApplyFailureFails(t *testing.T) {
	store := newFakeStore()
	agent := newFakeAgent()
	agent.applyErr["a"] = errors.New("boom")
	agent.healthy["a"] = true
	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1

	d, err := NewDeployment("pkg-1", "d1", "g1", "prod", "game-server", "game", 100, []string{"a", "b"}, "admin")
	require.NoError(t, err)
	require.NoError(t, store.Create(context.Background(), d))

	started, err := o.Start(context.Background(), d, pkg())
	require.Error(t, err)
	require.Equal(t, model.DeploymentStateFailed, started.State)
	targets := decodeTargets(started.TargetsJSON)
	require.Equal(t, model.DeploymentTargetFailed, targets[0].State)
}

func TestRollback(t *testing.T) {
	store := newFakeStore()
	agent := newFakeAgent()
	agent.healthy["a"] = true
	agent.healthy["b"] = true
	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1

	d, err := NewDeployment("pkg-1", "d1", "g1", "prod", "game-server", "game", 100, []string{"a", "b"}, "admin")
	require.NoError(t, err)
	require.NoError(t, store.Create(context.Background(), d))
	started, err := o.Start(context.Background(), d, pkg())
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateComplete, started.State)

	rbPkg := &model.DeployPackage{PackageID: "pkg-0", Version: "1.0.0", ObjectKey: "deploy/p/0.bin", Checksum: "999999999999"}
	rolled, err := o.Rollback(context.Background(), started, rbPkg)
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateRolledBack, rolled.State)
	targets := decodeTargets(rolled.TargetsJSON)
	for _, tg := range targets {
		require.Equal(t, model.DeploymentTargetRolledBack, tg.State)
	}
	require.Len(t, agent.applied, 4) // 2 apply + 2 rollback
}

func TestInvalidDeployment(t *testing.T) {
	_, err := NewDeployment("", "d", "", "", "", "", 100, []string{"a"}, "x")
	require.Error(t, err)
	_, err = NewDeployment("pkg", "d", "", "", "", "", 25, []string{"a"}, "x")
	require.Error(t, err)
	_, err = NewDeployment("pkg", "d", "", "", "", "", 100, nil, "x")
	require.Error(t, err)
}

func TestDeploymentIDUnique(t *testing.T) {
	a := NewDeploymentID()
	b := NewDeploymentID()
	require.NotEqual(t, a, b)
	_ = fmt.Sprintf("%s", a)
}
