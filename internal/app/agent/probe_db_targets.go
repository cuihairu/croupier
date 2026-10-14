package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/core/configsync"
	"github.com/cuihairu/croupier/core/healthprobe"
)

// ProbeDBTargetsFetch db 探针目标下行拉取 seam（生产经
// UpstreamClient.PullProbeDBTargets 走 control conn；测试注入）。version =
// 目标集内容 hash（非空；空集也有 hash）。
type ProbeDBTargetsFetch func(ctx context.Context) (*configsync.Versioned[string], error)

// ProbeDBTargetsPuller db 探针目标下行轮询（R4：周期 db 探测下放归属 agent
// 执行；server 按 db_sources.owner_agent_id 拉取时派生，不写
// config_versions、无推送通道）。
//
// 降级语义（诚实标注）：拉取失败保持最后已知目标集；空版本空正文 = 服务端
// 不可派生，同样保持现状（区别于采样策略的空版本=保持现状：本通道的空**集**
// 是真实状态——解除全部归属必须让 agent 卸掉动态探针，空集 hash 即其版本）。
type ProbeDBTargetsPuller struct {
	puller *configsync.Puller[string]
	pm     *ProbeManager
}

// NewProbeDBTargetsPuller 装配目标拉取器。interval<=0 取 configsync 默认
// 30s。fetch 缺一 PullOnce 报错；探针面缺一等价于无归属（拉取直接归一为
// 无下发，不算错误、不静默生效）。
func NewProbeDBTargetsPuller(pm *ProbeManager, fetch ProbeDBTargetsFetch, interval time.Duration, log *slog.Logger) *ProbeDBTargetsPuller {
	if log == nil {
		log = slog.Default()
	}
	p := &ProbeDBTargetsPuller{pm: pm}
	p.puller = configsync.New[string](interval,
		func(ctx context.Context) (*configsync.Versioned[string], error) {
			if fetch == nil {
				return nil, fmt.Errorf("probe db targets puller not initialized (fetch required)")
			}
			if p.pm == nil {
				return nil, nil
			}
			v, err := fetch(ctx)
			if err != nil {
				return nil, err
			}
			// 空版本 + 空正文 = 服务端不可派生：归一为无下发，保持现有目标。
			if v != nil && v.Version == "" && strings.TrimSpace(v.Payload) == "" {
				return nil, nil
			}
			return v, nil
		},
		func(_ context.Context, payload string) error {
			return p.applyTargets(payload)
		},
		configsync.WithOnError[string](func(err error) {
			log.Warn("probe db targets pull failed", "error", err)
		}),
		configsync.WithOnApply[string](func(version string) {
			log.Info("probe db targets applied", "version", version)
		}),
	)
	return p
}

// Start 启动轮询（立即一轮 + 周期循环，ctx 取消退出）。探针面未装配时空转
// 返回——没有探测面就没有目标可载。
func (p *ProbeDBTargetsPuller) Start(ctx context.Context) {
	if p == nil || p.pm == nil {
		return
	}
	p.puller.Start(ctx)
}

// PullOnce 执行一轮拉取与热生效；返回本轮是否实际应用了新版本与错误。
func (p *ProbeDBTargetsPuller) PullOnce(ctx context.Context) (bool, error) {
	if p == nil {
		return false, fmt.Errorf("probe db targets puller not initialized")
	}
	return p.puller.PullOnce(ctx)
}

// LastVersion 最近一次生效的目标集内容 hash（未生效过为空串）。
func (p *ProbeDBTargetsPuller) LastVersion() string {
	if p == nil {
		return ""
	}
	return p.puller.LastVersion()
}

// applyTargets 解析目标集 JSON 并 diff 装配。空正文视协议违约报错（版本游标
// 不推进，下轮重试）——空**集**合法（卸载全部动态探针），空**正文**不合法。
func (p *ProbeDBTargetsPuller) applyTargets(payload string) error {
	if strings.TrimSpace(payload) == "" {
		return fmt.Errorf("probe db targets: empty targets payload")
	}
	var cfgs []healthprobe.TargetConfig
	if err := json.Unmarshal([]byte(payload), &cfgs); err != nil {
		return fmt.Errorf("probe db targets: parse targets json: %w", err)
	}
	return p.pm.ApplyTargets(cfgs)
}
