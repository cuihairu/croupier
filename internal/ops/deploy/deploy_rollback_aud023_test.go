package deploy

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// AUD-023：回滚与正在执行的发版互斥。
//
// 发版单尚为 deploying 时不得回滚——此前 Rollback 无状态领取，在途批次推进
// 与回滚并发执行，且中间态目标（pending/deploying）被静默跳过。本用例把
// 发版单置于 deploying（批 1 a,b 已健康、批 2 c,d 仍 pending，模拟真实在途
// 形态），断言 Rollback 被拒、状态保持 deploying、目标不被误改、无任何回滚
// apply 被执行。
func TestRollback_RefusesWhileDeploying(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	agent.healthy["a"] = true
	agent.healthy["b"] = true
	o := New(store, agent)

	d, err := NewDeployment("pkg-1", "d-roll-inflight", "g1", "prod", "game-server", "game", 50, []string{"a", "b", "c", "d"}, "admin")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))

	// Start 的 CAS 路径：draft→deploying 已在途。
	ok, row, err := store.CompareAndSwapState(ctx, d.DeploymentID, model.DeploymentStateDraft, model.DeploymentStateDeploying)
	require.NoError(t, err)
	require.True(t, ok)

	// 在途形态：批 1（a,b）已换包健康，批 2（c,d）仍 pending。
	tgs := decodeTargets(row.TargetsJSON)
	tgs[0].State = model.DeploymentTargetHealthy
	tgs[1].State = model.DeploymentTargetHealthy
	require.NoError(t, saveTargets(row, tgs))
	require.NoError(t, store.Update(ctx, row))

	rbPkg := &model.DeployPackage{PackageID: "pkg-0", Version: "1.0.0", ObjectKey: "deploy/p/0.bin", Checksum: "999999999999"}
	got, err := o.Rollback(ctx, row, rbPkg)
	require.Error(t, err, "在途发版必须拒绝回滚")
	require.Contains(t, err.Error(), "cannot rollback while state is deploying")

	// 状态保持 deploying，目标未被误改，未执行任何回滚 apply。
	require.Equal(t, model.DeploymentStateDeploying, got.State)
	after := decodeTargets(got.TargetsJSON)
	require.Equal(t, model.DeploymentTargetHealthy, after[0].State)
	require.Equal(t, model.DeploymentTargetHealthy, after[1].State)
	require.Equal(t, model.DeploymentTargetPending, after[2].State)
	require.Equal(t, model.DeploymentTargetPending, after[3].State)
	require.Len(t, agent.applied, 0, "被拒的回滚不得执行任何 apply")
}

// AUD-023：零目标操作不得虚报整体 rolled_back。
//
// 无已应用/已失败目标（如 draft 全 pending、或目标已全为回滚态）时，此前
// Rollback 遍历跳过全部目标、返回 nil 并写 rolled_back——把「什么都没回滚」
// 虚报成成功。本用例断言 draft 全 pending 时回滚被拒、状态保持 draft、且
// 不写 rolled_back。
func TestRollback_ZeroTargetsNotFalseSuccess(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	o := New(store, agent)

	d, err := NewDeployment("pkg-1", "d-roll-zero", "g1", "prod", "game-server", "game", 100, []string{"a", "b"}, "admin")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))

	rbPkg := &model.DeployPackage{PackageID: "pkg-0", Version: "1.0.0", ObjectKey: "deploy/p/0.bin", Checksum: "999999999999"}
	got, err := o.Rollback(ctx, d, rbPkg)
	require.Error(t, err, "零目标回滚必须报错，不得虚报成功")
	require.Contains(t, err.Error(), "nothing to rollback")

	// 状态保持 draft（不得写成 rolled_back），目标全部未动。
	require.Equal(t, model.DeploymentStateDraft, got.State)
	require.Len(t, agent.applied, 0)
}

// AUD-023：重复回滚（目标已全为回滚态）同样零目标操作，必须报错而非再次
// 虚报成功或重复 apply。
func TestRollback_SecondRollbackAfterSuccessRejected(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	agent.healthy["a"] = true
	agent.healthy["b"] = true
	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1

	d, err := NewDeployment("pkg-1", "d-roll-twice", "g1", "prod", "game-server", "game", 100, []string{"a", "b"}, "admin")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))
	started, err := o.Start(ctx, d, pkg())
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateComplete, started.State)

	rbPkg := &model.DeployPackage{PackageID: "pkg-0", Version: "1.0.0", ObjectKey: "deploy/p/0.bin", Checksum: "999999999999"}
	rolled, err := o.Rollback(ctx, started, rbPkg)
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateRolledBack, rolled.State)
	require.Len(t, agent.applied, 4) // 2 apply + 2 rollback

	// 二次回滚：目标已全为 RolledBack → 零目标操作 → 报错，不再 apply。
	again, err := o.Rollback(ctx, rolled, rbPkg)
	require.Error(t, err, "重复回滚必须报错")
	require.Contains(t, err.Error(), "nothing to rollback")
	require.Equal(t, model.DeploymentStateRolledBack, again.State)
	require.Len(t, agent.applied, 4, "重复回滚不得再次执行 apply")
}
