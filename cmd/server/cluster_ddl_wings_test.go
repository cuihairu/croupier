// 覆盖率巡检第二十三轮（wt-api）：cmd/server cluster.go 三处翼——
// ① owner 表 EnsureTable 失败降级翼（115）：两连接构造——可写连接预建
//
//	成员表后以 mode=ro 重开，成员表 EnsureTable 幂等通过、owner 表
//	CREATE 被拒（推翻 coverage-exemptions.md cmd-5 中「单连接无法
//	构造」的论证，豁免项同步收窄为仅 NormalizeConfig）；
//
// ② reconcile 循环 Touch 失败 warn 翼（170）：BEFORE UPDATE 触发器
//
//	只拦 cluster_agent_owners——成员表心跳（Renew/Register）不受影响，
//	归属续期被拒后 last_seen_at 保持陈旧即为可观测后果；
//
// ③ 互联 Serve 出错 warn 翼（221）：不 Close 只 cancel ctx——Accept
//
//	deadline（1s）到期后 Serve 返回 ctx.Err()=Canceled 走错误日志
//	路径（既有用例先 srv.Close 走 closing 通道返回 nil，到不了本翼）。
//
// 另登记 refreshRemoteSnapshots 的 nil 会话 continue（439）防御不可达：
// LoadActiveSessions（registry/agent_session_db.go:104）逐行值扫描
// append，不产出 nil 元素。
package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/config"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 成员表已建成、owner 表建不上：只读连接上 AutoMigrate 对已存在的
// cluster_instances 幂等（纯 schema 读比对），对缺失的
// cluster_agent_owners CREATE 被拒 → standalone 降级。
func TestStartCluster_OwnerEnsureTableFailure(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "owner_ro.db")
	seed, err := gorm.Open(gsqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, cluster.NewDBMembership(seed, time.Second).EnsureTable(ctx))

	db, err := gorm.Open(gsqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled: true, InstanceID: "server-owner-ro", Store: "db",
		AdvertiseAddr: "127.0.0.1:0",
	}
	lc, srv := startCluster(ctx, c, &svc.ServiceContext{DB: db})
	assert.Nil(t, lc)
	assert.Nil(t, srv)
}

// reconcile 兜底循环的 Touch 失败翼：触发器拦归属表 UPDATE，成员表
// 写入（Register/Renew/INSERT）不受影响，集群正常启动、归属行不被续期。
func TestStartCluster_ReconcileTouchFailure(t *testing.T) {
	old := reconcileTickerInterval
	reconcileTickerInterval = 20 * time.Millisecond
	t.Cleanup(func() { reconcileTickerInterval = old })

	ctx := context.Background()
	svcCtx := newClusterSvcCtx(t)
	now := time.Now()
	require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-touchfail", GameID: "demo", Env: "prod",
		ExpireAt: now.Add(time.Hour), LastSeen: now,
	}))

	// 预建归属行（属本实例）→ 拨陈旧 → 挂触发器拦后续 UPDATE
	resolver := cluster.NewDBOwnerResolver(svcCtx.DB, 0)
	require.NoError(t, resolver.EnsureTable(ctx))
	require.NoError(t, resolver.ClaimOwner(ctx, "agent-touchfail", "demo", "prod", "server-touch", 1))
	stale := now.Add(-2 * time.Minute)
	require.NoError(t, svcCtx.DB.Exec(
		"UPDATE cluster_agent_owners SET last_seen_at = ? WHERE agent_id = ?",
		stale.UTC(), "agent-touchfail").Error)
	require.NoError(t, svcCtx.DB.Exec(
		`CREATE TRIGGER r23_touch_boom BEFORE UPDATE ON cluster_agent_owners `+
			`BEGIN SELECT RAISE(ABORT, 'r23 touch boom'); END`).Error)

	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled: true, InstanceID: "server-touch", Store: "db",
		AdvertiseAddr: "127.0.0.1:0", HeartbeatInterval: "200ms",
	}
	cctx, cancel := context.WithCancel(context.Background())
	lc, srv := startCluster(cctx, c, svcCtx)
	require.NotNil(t, lc)
	require.NotNil(t, srv)

	time.Sleep(300 * time.Millisecond) // 若干 tick 尝试 Touch 均被触发器拒绝

	var rec cluster.AgentOwnerRecord
	require.NoError(t, svcCtx.DB.Where("agent_id = ?", "agent-touchfail").First(&rec).Error)
	assert.True(t, rec.LastSeenAt.Before(stale.Add(time.Minute)),
		"Touch 被触发器拦截：last_seen_at 保持陈旧（续期失败的可观测后果）")

	cancel()
	stopCluster(t, lc, srv)
}

// 互联 Serve 的错误日志翼：cancel 而不 Close——Accept 的 1s deadline
// 到期后走 ctx.Done → 返回 Canceled（非 nil）→ goroutine 记 warn。
func TestStartCluster_InterconnectServeError(t *testing.T) {
	svcCtx := newClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled: true, InstanceID: "server-serve-err", Store: "db",
		AdvertiseAddr: "127.0.0.1:0", HeartbeatInterval: "200ms",
	}
	ctx, cancel := context.WithCancel(context.Background())
	lc, srv := startCluster(ctx, c, svcCtx)
	require.NotNil(t, lc)
	require.NotNil(t, srv)

	cancel()
	time.Sleep(1500 * time.Millisecond) // > Accept deadline：Serve 已带 ctx.Err() 返回

	stopCluster(t, lc, srv) // srv.Close 后置无害（Serve 已退出）
}
