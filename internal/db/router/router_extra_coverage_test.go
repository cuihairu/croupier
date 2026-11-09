package router

// router.go 补测：Stats 的冒泡排序交换臂（注入乱序时长）、recordFirstOpen
// 的环形覆写臂（窗口满 1024）、Forget 的未命中 continue 臂、openGameDB 的
// ensure/open 阶段取消臂（取消上下文与 EnsureDatabase 内取消父上下文）、
// ping 失败臂（关闭底层 sql.DB 使 PingContext 报错）。
//
// 诚实跳过：Stats 的 P95 下标钳位（338——313 空守卫保证 len>=1，且
// int(0.95*len) < len 恒成立，钳位不可达）；openGameDB 的 migrate 阶段
// 取消臂（500）——唯一确定性取消源是预算超时，若预算在 ping 期间到期，
// PingContext 先感知 ctx 取消并报 ping 错（495），ping 成功与
// check("migrate") 之间的取消窗口为零宽竞态，无法确定性注入。

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRouterStatsBubbleSortSwapArm(t *testing.T) {
	r := &Router{}
	// 乱序时长：冒泡排序必须交换才会命中 332 交换臂。
	r.openDurations = []time.Duration{3 * time.Second, 1 * time.Second, 2 * time.Second}
	s := r.Stats()
	require.Equal(t, 3, s.Window)
	require.Equal(t, 2*time.Second, s.AvgLatency)
	require.Equal(t, 3*time.Second, s.MaxLatency)
	require.Equal(t, 3*time.Second, s.P95Latency)
}

func TestRouterRecordFirstOpenRingOverwrite(t *testing.T) {
	r := &Router{}
	r.openDurations = make([]time.Duration, maxOpenStatsWindow)
	r.recordFirstOpen(7 * time.Second)
	require.Len(t, r.openDurations, maxOpenStatsWindow)
	// 环形右移后最新样本落在末位（copy 覆写臂命中）。
	require.Equal(t, 7*time.Second, r.openDurations[maxOpenStatsWindow-1])
	require.Equal(t, time.Duration(0), r.openDurations[0])
}

func TestRouterForgetNonMatchingContinue(t *testing.T) {
	r := &Router{
		cache:    map[string]*gorm.DB{"g1_prod": nil},
		gameOfDB: map[string]scopeKey{"g1_prod": {gameID: "g1", env: "prod"}},
	}
	// 目标 scope 与缓存条目不同 → 循环 continue（410），无匹配返回 nil。
	require.NoError(t, r.Forget("g2", "prod"))
}

func TestOpenGameDBCancelledAtEnsure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&Router{}).openGameDB(ctx, "gamedb")
	require.ErrorContains(t, err, "cancelled")
	require.ErrorContains(t, err, "ensure")
}

func TestOpenGameDBCancelledAtOpen(t *testing.T) {
	parentCtx, cancel := context.WithCancel(context.Background())
	r := &Router{}
	r.cfg = Config{
		Driver: "sqlite",
		DSNForDatabase: func(metaDSN, dbName string) string {
			return dbName
		},
		// 在 ensure 阶段内取消父上下文 → 派生的预算子上下文随之取消 →
		// check("open") 感知取消（479）。
		EnsureDatabase: func(ctx context.Context, driver, metaDSN, dbName string) (string, error) {
			cancel()
			return "dsn", nil
		},
	}
	_, err := r.openGameDB(parentCtx, "gamedb")
	require.ErrorContains(t, err, "cancelled")
	require.ErrorContains(t, err, "open")
}

func TestOpenGameDBPingError(t *testing.T) {
	closed := openSQLiteDB(t, "file:router_ping?mode=memory&cache=shared")
	sqlDB, err := closed.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	r := &Router{}
	r.cfg = Config{
		Driver: "sqlite",
		DSNForDatabase: func(metaDSN, dbName string) string {
			return dbName
		},
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return closed, nil
		},
	}
	_, err = r.openGameDB(context.Background(), "gamedb")
	require.ErrorContains(t, err, "ping database")
}
