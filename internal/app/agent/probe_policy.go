package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/core/configsync"
	"github.com/cuihairu/croupier/core/healthprobe"
)

// ProbePolicyFetch 采样策略下行拉取 seam（生产经 UpstreamClient.PullProbePolicy
// 走 control conn；测试注入）。返回 version 空串 = 服务端未发布。
type ProbePolicyFetch func(ctx context.Context) (*configsync.Versioned[string], error)

// ProbePolicyPuller probe.policy 采样策略下行轮询（评审简档 §13.3 C）：
// config_versions 资产 → server 按 agent 会话 scope 读 → agent 周期拉取 →
// 热更生效（间隔函数读侧现取，下轮即跟随，无需重启探针）。
//
// 降级语义（诚实标注）：拉取失败或策略解析失败一律保持最后已知策略；服务端
// 未发布（空版本空内容）保持现状，不清空已生效的策略。
type ProbePolicyPuller struct {
	puller *configsync.Puller[string]
	pm     *ProbeManager
}

// NewProbePolicyPuller 装配采样策略拉取器。interval<=0 取 configsync 默认
// 30s。fetch 缺一 PullOnce 报错；探针面缺一等价于未发布（拉取直接归一为
// 无下发，不算错误、不静默生效）。
func NewProbePolicyPuller(pm *ProbeManager, fetch ProbePolicyFetch, interval time.Duration, log *slog.Logger) *ProbePolicyPuller {
	if log == nil {
		log = slog.Default()
	}
	p := &ProbePolicyPuller{pm: pm}
	p.puller = configsync.New[string](interval,
		func(ctx context.Context) (*configsync.Versioned[string], error) {
			if fetch == nil {
				return nil, fmt.Errorf("probe policy puller not initialized (fetch required)")
			}
			if p.pm == nil {
				return nil, nil
			}
			v, err := fetch(ctx)
			if err != nil {
				return nil, err
			}
			// 空版本空内容 = 服务端未发布：归一为无下发，不触发 apply/onApply。
			if v != nil && v.Version == "" && strings.TrimSpace(v.Payload) == "" {
				return nil, nil
			}
			return v, nil
		},
		func(_ context.Context, payload string) error {
			return p.applyPolicy(payload)
		},
		configsync.WithOnError[string](func(err error) {
			log.Warn("probe policy pull failed", "error", err)
		}),
		configsync.WithOnApply[string](func(version string) {
			log.Info("probe policy applied", "version", version)
		}),
	)
	return p
}

// Start 启动轮询（立即一轮 + 周期循环，ctx 取消退出）。探针面未装配时空转
// 返回——没有探测就没有采样频率可言。
func (p *ProbePolicyPuller) Start(ctx context.Context) {
	if p == nil || p.pm == nil {
		return
	}
	p.puller.Start(ctx)
}

// PullOnce 执行一轮拉取与热生效；返回本轮是否实际应用了新版本与错误。
func (p *ProbePolicyPuller) PullOnce(ctx context.Context) (bool, error) {
	if p == nil {
		return false, fmt.Errorf("probe policy puller not initialized")
	}
	return p.puller.PullOnce(ctx)
}

// LastVersion 最近一次生效的策略版本号（未生效过为空串）。
func (p *ProbePolicyPuller) LastVersion() string {
	if p == nil {
		return ""
	}
	return p.puller.LastVersion()
}

// applyPolicy 校验并热更策略快照（与面板发布预检共用 core/healthprobe 解析
// 真值——同款拒绝一份坏策略在两侧产生分歧）。空内容 = 未发布，保持现状。
func (p *ProbePolicyPuller) applyPolicy(payload string) error {
	if strings.TrimSpace(payload) == "" {
		return nil
	}
	policy, err := healthprobe.ParseProbePolicy(payload)
	if err != nil {
		return err
	}
	p.pm.SetPolicy(policy)
	return nil
}
