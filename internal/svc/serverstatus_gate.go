package svc

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"github.com/cuihairu/croupier/internal/platform/serverstatus"
)

// wireServerStatusGate 把 serverStatus 配置接成出口链维护状态 gate 钩子
// （server-status-provider 设计 §5）：registerOutlets 收尾调用，enabled=true
// 且 provider 已注册才挂 gate，否则不挂——零配置=零行为变化（全部照报）。
func wireServerStatusGate(ctx *ServiceContext) {
	if ctx == nil || ctx.OutletManager == nil {
		return
	}
	installServerStatusGate(ctx.Config, ctx.OutletManager)
}

// installServerStatusGate 纯装配（可测）：config→serverstatus.Options 映射
// + NewGate + manager.SetGate。matchBy 非 agentId（host/ip）时 v1 事件
// scope 只带 agentId、agent_sessions 无主机列可 join——回落 agentId 并启动
// 告警（不静默忽略配置）。token 走 tokenEnv 环境变量解析（凭证不落配置
// 文件——herald 同款口径）。
func installServerStatusGate(cfg config.Config, manager *outlet.Manager) {
	scfg := cfg.ServerStatus
	if !scfg.Enabled {
		return
	}
	providerKey := strings.TrimSpace(scfg.Provider)
	factory := serverstatus.Lookup(providerKey)
	if factory == nil {
		slog.WarnContext(context.Background(), "serverStatus enabled but provider not registered; maintenance gate bypassed",
			"provider", scfg.Provider)
		return
	}
	matchKey := strings.TrimSpace(scfg.MatchBy)
	if matchKey != "agentId" {
		slog.WarnContext(context.Background(), "serverStatus.matchBy host/ip not resolvable from v1 events (scope carries agentId only); falling back to agentId",
			"matchBy", scfg.MatchBy)
		matchKey = "agentId"
	}
	source := scfg.Providers[providerKey]
	opts := serverstatus.Options{
		BaseURL: source.BaseURL,
		Token:   os.Getenv(source.TokenEnv),
		Timeout: time.Duration(source.TimeoutMs) * time.Millisecond,
		Atlas: serverstatus.AtlasOptions{
			StatusPath: source.StatusPath,
			MatchKey:   matchKey,
			Fields:     source.FieldMapping,
		},
	}
	gate := serverstatus.NewGate(factory(opts),
		serverstatus.NewCache(time.Duration(scfg.CacheTtlSeconds)*time.Second), nil)
	manager.SetGate(func(ctx context.Context, ev outlet.AlertEvent) (bool, map[string]string) {
		v := gate.Evaluate(ctx, serverstatus.ServerRef{
			AgentID: ev.Scope.AgentID,
			GameID:  ev.Scope.GameID,
			Env:     ev.Scope.Env,
		})
		if !v.Suppressed && !v.SourceUnknown {
			return false, nil
		}
		meta := make(map[string]string, 2)
		if v.Suppressed {
			meta[outlet.MetaMaintenanceSuppressed] = "true"
		}
		if v.SourceUnknown {
			meta[outlet.MetaSourceUnknown] = "true"
		}
		return v.Suppressed, meta
	})
	slog.InfoContext(context.Background(), "serverStatus maintenance gate wired",
		"provider", providerKey, "matchBy", matchKey, "cacheTtlSeconds", scfg.CacheTtlSeconds)
}
