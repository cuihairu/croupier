package server

// probe_lifecycle.go 补测：onWindow 的 nil 接收者/未装配告警模型/nil window
// 载荷守卫、不支持 window_kind 告警、firing/resolved 转换落库失败的告警
// 降级（生命周期是消费面，缺装配/落库失败不得放大为通道错误）、recovered
// 重放的 resolved 幂等、recentFailures 的 nil 接收者/nil sink 守卫。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	gosqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log/slog"
)

// brokenAlertsLifecycle alerts 表缺失的生命周期（落库失败告警面）。
func brokenAlertsLifecycle(t *testing.T) *probeLifecycle {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Alert{}))
	require.NoError(t, db.Migrator().DropTable("alerts"))
	return newProbeLifecycle(model.NewAlertModel(db), nil, slog.Default())
}

func TestProbeLifecycleNilGuards(t *testing.T) {
	ctx := context.Background()

	// nil 接收者 → 直接返回
	var nilLifecycle *probeLifecycle
	nilLifecycle.onWindow(ctx, "agent-1", probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0))
	require.Nil(t, nilLifecycle.recentFailures("agent-1", "redis:6379", 0))

	// alerts 未装配 → 静默跳过
	unwired := newProbeLifecycle(nil, nil, slog.Default())
	unwired.onWindow(ctx, "agent-1", probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0))

	// nil window 载荷 → 告警返回（alerts 需在册，否则前置守卫先返回）
	_, wiredAlerts := newProbeLifecycleTestService(t)
	wired := newProbeLifecycle(wiredAlerts, nil, slog.Default())
	wired.onWindow(ctx, "agent-1", &opsv1.AlertEvent{Kind: "probe.window"})

	// 不支持的 window_kind → 告警返回
	svc, _ := newProbeLifecycleTestService(t)
	_, err := svc.handleProbeEvent(ctx, marshalAlertEvent(t,
		probeWindowEvent("redis:6379", "flapping", 1000, 0, 0)))
	require.NoError(t, err, "unsupported window kind must not fail the channel")

	// recentFailures 的 nil sink 守卫
	require.Nil(t, wired.recentFailures("agent-1", "redis:6379", 0))
}

func TestProbeLifecycleTransitionFailWarns(t *testing.T) {
	ctx := context.Background()
	l := brokenAlertsLifecycle(t)

	// firing 转换落库失败 → 告警降级，不放大为错误
	l.onWindow(ctx, "agent-1", probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0))
	// resolved 转换落库失败 → 同上
	l.onWindow(ctx, "agent-1", probeWindowEvent("redis:6379", "recovered", 1000, 13000, 12000))
}

func TestProbeLifecycleResolvedReplayIdempotent(t *testing.T) {
	svc, alertModel := newProbeLifecycleTestService(t)
	ctx := context.Background()

	_, err := svc.handleProbeEvent(ctx, marshalAlertEvent(t,
		probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0)))
	require.NoError(t, err)
	_, err = svc.handleProbeEvent(ctx, marshalAlertEvent(t,
		probeWindowEvent("redis:6379", "recovered", 1000, 13000, 12000)))
	require.NoError(t, err)
	// recovered 重放（重连重放）→ resolved 幂等，不动已终态行
	_, err = svc.handleProbeEvent(ctx, marshalAlertEvent(t,
		probeWindowEvent("redis:6379", "recovered", 1000, 13000, 12000)))
	require.NoError(t, err)

	row, err := alertModel.FindByAlertID(ctx, probeAlertID("agent-1", "redis:6379"))
	require.NoError(t, err)
	require.Equal(t, "resolved", row.Status)
}
