package deploy

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// AUD-022：可取消的批次等待前先持久化已完成目标。
//
// 编排器在「当前批全部健康」后、进入 BatchWait 等待前，必须先落库本批目标
// 事实——此前 saveTargets/Update 只在等待之后执行，批次等待中被取消时已换
// 包/已健康的目标仍留在 DB 的 pending/deploying，被静默当作未完成而重放。
// 本用例直接驱动编排器：批 1（a,b）换包完成、第二台机健康确认点取消 ctx，
// 断言取消返回后存储仍保留「已健康」事实且未执行批不被误标。
func TestAdvance_CancelDuringBatchWaitPersistsCompletedTargets(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	agent.healthy["a"] = true
	agent.healthy["b"] = true

	d, err := NewDeployment("pkg-1", "d-cancel", "g1", "prod", "game-server", "game", 50, []string{"a", "b", "c", "d"}, "admin")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))

	// 第二台机健康确认时取消 ctx：批 1（a,b）换包完成后立刻进入批次等待。
	waitCtx, cancel := context.WithCancel(ctx)
	agent.onHealthy = func(agentID string) {
		if agentID == "b" {
			cancel()
		}
	}

	o := New(store, agent)
	o.BatchWait = 10 * time.Second // 等待未被消费（取消即返回）
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1

	_, err = o.Start(waitCtx, d, pkg())
	require.ErrorIs(t, err, context.Canceled)

	// 取消返回后：已换包/已健康的目标不得退回 pending（AUD-022 主断言）。
	got, err := store.FindByID(ctx, d.DeploymentID)
	require.NoError(t, err)
	targets := decodeTargets(got.TargetsJSON)
	require.Len(t, targets, 4)
	require.Equal(t, model.DeploymentTargetHealthy, targets[0].State, "取消点前已健康目标必须持久化")
	require.Equal(t, model.DeploymentTargetHealthy, targets[1].State, "取消点前已健康目标必须持久化")
	require.Equal(t, model.DeploymentTargetPending, targets[2].State, "未执行批不得被误标")
	require.Equal(t, model.DeploymentTargetPending, targets[3].State)
	require.Equal(t, model.DeploymentStateDeploying, got.State, "取消后发版单保持 deploying，可继续收敛")

	// 收敛策略验证：再次 Advance 跳过已健康批、推进下一批（c,d），不重放 a,b。
	agent.healthy["c"] = true
	agent.healthy["d"] = true
	o.BatchWait = 0
	resumed, err := o.Advance(ctx, got, pkg())
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateComplete, resumed.State)
	applied := decodeTargets(resumed.TargetsJSON)
	require.Equal(t, model.DeploymentTargetHealthy, applied[0].State)
	require.Equal(t, model.DeploymentTargetHealthy, applied[1].State)
	require.Equal(t, model.DeploymentTargetHealthy, applied[2].State)
	require.Equal(t, model.DeploymentTargetHealthy, applied[3].State)
}
