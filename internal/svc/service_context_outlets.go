package svc

import (
	"context"
	"os"
	"strconv"
	"strings"

	"log/slog"

	"github.com/cuihairu/croupier/internal/platform/outlet"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// registerOutlets 布线告警出口链（plugin-mechanism §5.1）：站内出口链首恒
// 注册（默认必有、零外发、不受降级影响）；herald 显式启用且凭证齐全时
// 注册为外部主出口，否则注册 noop——零配置=零外发红线的真实注册项。
// MetricsStore 缺失（裁剪部署）时不接线——出口只经 supervisor hook 触达。
func registerOutlets(ctx *ServiceContext) {
	if ctx.MetricsStore == nil {
		return
	}
	manager := outlet.NewManager()
	manager.Register(outlet.NewInternalOutlet(messageSink{db: ctx.DB}, outlet.DefaultStationRecipient))
	cfg := ctx.Config.Herald
	external := false
	if cfg.Enabled && strings.TrimSpace(cfg.BaseURL) != "" {
		tokenEnv := cfg.TokenEnv
		if tokenEnv == "" {
			tokenEnv = outlet.HeraldDefaultTokenEnv
		}
		token := os.Getenv(tokenEnv)
		app := cfg.App
		if app == "" {
			app = outlet.HeraldDefaultApp
		}
		target := cfg.Target
		if target == "" {
			target = outlet.HeraldDefaultTarget
		}
		if token == "" {
			slog.WarnContext(context.Background(), "herald outlet enabled but token env missing; falling back to noop",
				"tokenEnv", tokenEnv)
		} else {
			manager.Register(outlet.NewHeraldOutlet(cfg.BaseURL, app, token, target))
			external = true
			slog.InfoContext(context.Background(), "herald outlet registered", "app", app, "target", target)
		}
	}
	if !external {
		manager.Register(outlet.NewNoopOutlet())
	}
	ctx.OutletManager = manager
	ctx.StationSink = messageSink{db: ctx.DB}
	ctx.MetricsStore.SetOnSupervisorEvent(supervisorEventToOutlet(manager))
	wireServerStatusGate(ctx)
}

// supervisorEventToOutlet 把新监管事件映射为统一信封：v1 闭集只消费
// breaker_tripped（severity 恒 critical；其余类型出口零改动地忽略）。
// event_id/dedup_key 携 agentId——seq 是 agent 本地游标、重启归零，跨
// agent 不加前缀必撞。
func supervisorEventToOutlet(m *outlet.Manager) func(ctx context.Context, agentID string, ev *opsv1.SupervisorEvent) {
	return func(_ context.Context, agentID string, ev *opsv1.SupervisorEvent) {
		if m == nil || ev == nil || ev.GetEvent() != "breaker_tripped" {
			return
		}
		title := "supervisor: breaker tripped for " + ev.GetProcess()
		body := ev.GetMessage()
		if body == "" {
			body = "too many restart failures"
		}
		m.Dispatch(outlet.AlertEvent{
			Kind:             outlet.KindSupervisorBreaker,
			Severity:         outlet.SeverityCritical,
			EventID:          agentID + ":" + strconv.FormatInt(ev.GetSeq(), 10),
			DedupKey:         "supervisor.breaker_tripped:" + agentID + ":" + strconv.FormatInt(ev.GetSeq(), 10),
			Title:            title,
			Body:             body,
			OccurredAtUnixMs: ev.GetTsUnix() * 1000,
			Scope:            outlet.Scope{AgentID: agentID},
		})
	}
}
