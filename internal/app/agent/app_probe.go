package agent

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/core/configsync"
)

// WithProbeConfig 配置声明式探针（`probe.targets[]`，R2）。校验失败即返回错误
// 且不起装配——配置错却在运行期静默少探，等于把观测面悄悄关掉。targets 为
// 空也起装配（R4：空静态集 = 只接 server 派生的 db 目标）。
func (a *App) WithProbeConfig(cfg *ProbeConfig) error {
	if a == nil || cfg == nil {
		return nil
	}
	if a.upstream == nil {
		return errors.New("agent upstream not initialized")
	}
	pm, err := NewProbeManager(cfg.Targets, ProbeManagerOptions{
		AgentID:             a.agentID,
		GameID:              a.upstream.GameID(),
		Env:                 a.upstream.Env(),
		Send:                a.upstream.AlertEventSender(),
		MaxDetailsPerWindow: cfg.MaxDetailsPerWindow,
	})
	if err != nil {
		return err
	}
	a.probeManager = pm
	// 正常态统计折进 metrics 批量面（§13.1：探测统计与系统指标同面捎带）。
	a.upstream.WithProbeSampler(pm)
	return nil
}

// ProbeManager 返回已装配的探针面（未配置时 nil）。
func (a *App) ProbeManager() *ProbeManager {
	if a == nil {
		return nil
	}
	return a.probeManager
}

// startProbes 启动周期探测（每个目标一个 goroutine，随 ctx 取消退出）。
func (a *App) startProbes(ctx context.Context) {
	if a == nil || a.probeManager == nil {
		return
	}
	go a.probeManager.Start(ctx)
}

// startProbePolicy 启动采样策略下行轮询（probe.policy，§13.3 C）。未配置
// 探针面时不启动；未就绪期间按本地配置间隔跑，策略到达后热更。
func (a *App) startProbePolicy(ctx context.Context) {
	if a == nil || a.probeManager == nil || a.upstream == nil {
		return
	}
	NewProbePolicyPuller(a.probeManager, a.fetchProbePolicy, configsync.DefaultInterval, nil).Start(ctx)
}

// fetchProbePolicy 采样策略拉取（经 control conn 的 MsgProbePolicyRequest；
// version 空串 = 未发布，Puller 保持现状）。
func (a *App) fetchProbePolicy(ctx context.Context) (*configsync.Versioned[string], error) {
	resp, err := a.upstream.PullProbePolicy(ctx)
	if err != nil {
		return nil, err
	}
	return &configsync.Versioned[string]{Version: resp.GetVersion(), Payload: resp.GetPolicyJson()}, nil
}

// startProbeDBTargets 启动 db 探针目标下行轮询（R4：server 按
// db_sources.owner_agent_id 归属派生；探测面未装配时不启动）。
func (a *App) startProbeDBTargets(ctx context.Context) {
	if a == nil || a.probeManager == nil || a.upstream == nil {
		return
	}
	NewProbeDBTargetsPuller(a.probeManager, a.fetchProbeDBTargets, configsync.DefaultInterval, nil).Start(ctx)
}

// fetchProbeDBTargets db 目标拉取（经 control conn 的 MsgProbeDBTargetsRequest；
// version = 目标集内容 hash，空集也有 hash）。
func (a *App) fetchProbeDBTargets(ctx context.Context) (*configsync.Versioned[string], error) {
	resp, err := a.upstream.PullProbeDBTargets(ctx)
	if err != nil {
		return nil, err
	}
	return &configsync.Versioned[string]{Version: resp.GetVersion(), Payload: resp.GetTargetsJson()}, nil
}
