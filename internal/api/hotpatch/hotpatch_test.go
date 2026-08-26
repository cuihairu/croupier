package hotpatch

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var hpSeq uint64

func newFixture(t *testing.T) *Service {
	t.Helper()
	name := fmt.Sprintf("hp_%d", atomic.AddUint64(&hpSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	return NewService(&svc.ServiceContext{
		HotpatchModel: model.NewHotpatchModel(db),
		ObjectStore:   store,
	})
}

func seedDraft(t *testing.T, svc *Service) *Hotpatch {
	t.Helper()
	res, err := svc.Create(context.Background(), &CreateRequest{
		GameID: "demo", Env: "prod", Framework: model.HotpatchFrameworkSkynet,
		BugID: 42, Title: "修复背包闪退",
	})
	require.NoError(t, err)
	return &res.Hotpatch
}

func TestHotpatchLifecycle_WithGuards(t *testing.T) {
	ctx := context.Background()
	svc := newFixture(t)
	hp := seedDraft(t, svc)

	// Approve without package → rejected.
	_, err := svc.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "approve"})
	require.ErrorContains(t, err, "package")

	// Create without bug → rejected at create time.
	_, err = svc.Create(ctx, &CreateRequest{GameID: "demo", Framework: "skynet", Title: "x"})
	require.ErrorContains(t, err, "缺陷")

	// Bad framework → rejected.
	_, err = svc.Create(ctx, &CreateRequest{GameID: "demo", Framework: "lua", BugID: 1, Title: "x"})
	require.ErrorContains(t, err, "框架")

	// Upload then approve → roll(30) → roll(70) → applied.
	_, err = svc.UploadPackage(ctx, &UploadRequest{
		ID: fmt.Sprint(hp.Id), Data: strings.NewReader("patch-bytes"), Size: 12,
		ContentType: "application/octet-stream",
	})
	require.NoError(t, err)

	_, err = svc.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "approve"})
	require.NoError(t, err)

	pct := 30
	_, err = svc.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "roll", RolloutPercent: &pct})
	require.NoError(t, err)
	lower := 10
	_, err = svc.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "roll", RolloutPercent: &lower})
	require.ErrorContains(t, err, "only increase")
	pct = 70
	_, err = svc.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "roll", RolloutPercent: &pct})
	require.NoError(t, err)

	_, err = svc.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "applied"})
	require.NoError(t, err)

	// Applied can only rollback.
	_, err = svc.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "roll", RolloutPercent: &pct})
	require.ErrorContains(t, err, "invalid transition")

	// Agent result recorded (last wins per agent).
	require.NoError(t, svc.ReportResult(ctx, &ResultRequest{
		ID: fmt.Sprint(hp.Id), AgentID: "agent-1", Node: "node-a", Status: "ok", Log: "inject ok",
	}))
	require.NoError(t, svc.ReportResult(ctx, &ResultRequest{
		ID: fmt.Sprint(hp.Id), AgentID: "agent-1", Node: "node-a", Status: "failed", Log: "verify failed",
	}))
	list, err := svc.List(ctx, &ListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Len(t, list.Items[0].Results, 1)
	assert.Equal(t, "failed", list.Items[0].Results[0].Status)

	// BucketHit stability.
	assert.True(t, (&model.Hotpatch{RolloutPercent: 100}).BucketHit("n-1"))
	assert.False(t, (&model.Hotpatch{RolloutPercent: 0}).BucketHit("n-1"))
}

func TestBucketHitDeterministic(t *testing.T) {
	hp := &model.Hotpatch{RolloutPercent: 50, RolloutSeed: "seed-x"}
	first := hp.BucketHit("device-9")
	for range 20 {
		assert.Equal(t, first, hp.BucketHit("device-9"))
	}
}
