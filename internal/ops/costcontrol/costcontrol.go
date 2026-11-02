// Package costcontrol implements OPS-007 cost estimation, node cost tagging,
// idle reclamation and budget alerting.
package costcontrol

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

// HourlyRates 规格价格目录（USD/小时，跨 provider 归一化）。生产价需接入
// 真实账单/价格 API；本目录为内置参考价（诚实：估算精度登记为部署后对账）。
var HourlyRates = map[string]map[string]float64{
	"aliyun":  {"ecs.s6-c1.m1.large": 0.050, "ecs.g6.large": 0.10, "ecs.c6.large": 0.09},
	"tencent": {"S5.MEDIUM4": 0.048, "S5.LARGE8": 0.09},
	"aws":     {"t3.micro": 0.0104, "t3.small": 0.0208, "t3.medium": 0.0416},
	"hetzner": {"cx21": 0.006, "cx31": 0.012, "cx41": 0.024},
}

// Rate lookup helper.
func Rate(provider, instanceType string) (float64, bool) {
	m, ok := HourlyRates[provider]
	if !ok {
		return 0, false
	}
	r, ok := m[instanceType]
	return r, ok
}

// EstimateInput 创建前估算。
type EstimateInput struct {
	Provider     string `json:"provider"`
	InstanceType string `json:"instanceType"`
	Count        int    `json:"count"`
	// Hours 运行时长（小时）。0 = 默认 1 小时；>0 按实际。
	Hours float64 `json:"hours"`
	// DiskSizeGB 附加系统盘（每 GB 每月固定价 0.08 USD，按小时折算）。
	DiskSizeGB int `json:"diskSizeGB"`
}

// Estimate 估算结果。
type Estimate struct {
	Provider     string  `json:"provider"`
	InstanceType string  `json:"instanceType"`
	Count        int     `json:"count"`
	Hours        float64 `json:"hours"`
	RatePerHour  float64 `json:"ratePerHour"`
	DiskPerHour  float64 `json:"diskPerHour"`
	TotalUSD     float64 `json:"totalUsd"`
	KnownRate    bool    `json:"knownRate"`
}

// ComputeEstimate computes a pre-creation cost estimate.
func ComputeEstimate(in EstimateInput) (*Estimate, error) {
	if in.Provider == "" || in.InstanceType == "" {
		return nil, errors.New("provider and instanceType required")
	}
	if in.Count <= 0 {
		return nil, errors.New("count must be positive")
	}
	if in.Hours <= 0 {
		in.Hours = 1
	}
	rate, known := Rate(in.Provider, in.InstanceType)
	diskPerHour := 0.0
	if in.DiskSizeGB > 0 {
		diskPerHour = float64(in.DiskSizeGB) * 0.08 / 720.0 // monthly / 720h
	}
	total := (rate + diskPerHour) * float64(in.Count) * in.Hours
	return &Estimate{
		Provider: in.Provider, InstanceType: in.InstanceType, Count: in.Count,
		Hours: in.Hours, RatePerHour: rate, DiskPerHour: diskPerHour,
		TotalUSD: total, KnownRate: known,
	}, nil
}

// NodeCost 单机成本标签（由 provision run 规格 + 运行时长 + 价格目录推导）。
type NodeCost struct {
	AgentID      string `json:"agentId"`
	Provider     string `json:"provider"`
	InstanceType string `json:"instanceType"`
	Region       string `json:"region"`
	// UptimeHours 运行时长（小时，基于 run CreatedAt）。
	UptimeHours float64 `json:"uptimeHours"`
	RatePerHour float64 `json:"ratePerHour"`
	CostUSD     float64 `json:"costUsd"`
	// Idle 是否满足闲置判定（由调用方传入低负载标志）。
	Idle bool `json:"idle"`
	// IdleHours 持续低负载时长（由调用方传入，0=未知）。
	IdleHours float64 `json:"idleHours"`
}

// Summary 总览。
type Summary struct {
	TotalNodes  int                `json:"totalNodes"`
	TotalUSD    float64            `json:"totalUsd"`
	PerProvider map[string]float64 `json:"perProvider"`
	IdleCount   int                `json:"idleCount"`
	Nodes       []NodeCost         `json:"nodes"`
}

// BuildSummary aggregates node costs from provision runs + idle flags.
func BuildSummary(runs []model.ProvisionRun, idle map[string]bool, idleHours map[string]float64, now time.Time) *Summary {
	s := &Summary{PerProvider: map[string]float64{}}
	for i := range runs {
		r := &runs[i]
		var spec struct {
			InstanceType string `json:"instanceType"`
		}
		_ = json.Unmarshal(r.SpecJSON, &spec)
		rate, known := Rate(r.Provider, spec.InstanceType)
		uptime := now.Sub(r.CreatedAt).Hours()
		if uptime < 0 {
			uptime = 0
		}
		cost := rate * uptime
		if !known {
			cost = 0 // 未知规格不计入（诚实：不伪造价）
		}
		agentID := ""
		var agents []string
		_ = json.Unmarshal(r.AgentIDsJSON, &agents)
		if len(agents) > 0 {
			agentID = agents[0]
		}
		nc := NodeCost{
			AgentID: agentID, Provider: r.Provider, InstanceType: spec.InstanceType,
			Region: r.Region, UptimeHours: uptime, RatePerHour: rate, CostUSD: cost,
			Idle: idle[agentID], IdleHours: idleHours[agentID],
		}
		s.Nodes = append(s.Nodes, nc)
		s.TotalNodes++
		s.TotalUSD += cost
		s.PerProvider[r.Provider] += cost
		if nc.Idle {
			s.IdleCount++
		}
	}
	sort.Slice(s.Nodes, func(a, b int) bool { return s.Nodes[a].CostUSD > s.Nodes[b].CostUSD })
	return s
}

// Budget 月度预算。
type Budget struct {
	GameID         string    `json:"gameId"`
	Env            string    `json:"env"`
	MonthlyUSD     float64   `json:"monthlyUsd"`
	NotifyChannels []string  `json:"notifyChannels"`
	UpdatedBy      string    `json:"updatedBy"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// IdlePolicy 闲置回收策略（可配）。
type IdlePolicy struct {
	// Enabled 自动回收总开关。默认 false（只提醒不销毁）。
	Enabled bool `json:"enabled"`
	// IdleHours 无负载判定阈值（小时）。默认 24。
	IdleHours float64 `json:"idleHours"`
	// CPUThreshold 低负载 CPU 判定（< 阈值 %）。默认 5。
	CPUThreshold float64 `json:"cpuThreshold"`
	// AutoDestroy 到达阈值后自动销毁（需 Enabled=true 且未防护）。
	AutoDestroy bool `json:"autoDestroy"`
}

// DefaultIdlePolicy returns the safe default (remind only).
func DefaultIdlePolicy() IdlePolicy {
	return IdlePolicy{IdleHours: 24, CPUThreshold: 5}
}

// Normalize fills defaults.
func (p *IdlePolicy) Normalize() {
	if p.IdleHours <= 0 {
		p.IdleHours = 24
	}
	if p.CPUThreshold <= 0 {
		p.CPUThreshold = 5
	}
}

// Validate checks bounds.
func (p IdlePolicy) Validate() error {
	if p.IdleHours < 1 || p.IdleHours > 24*90 {
		return fmt.Errorf("idleHours must be 1..2160")
	}
	if p.CPUThreshold < 0 || p.CPUThreshold > 100 {
		return fmt.Errorf("cpuThreshold must be 0..100")
	}
	return nil
}

// FormatProviders lists known providers.
func FormatProviders() []string {
	var out []string
	for k := range HourlyRates {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// NormalizeProvider lowercases and trims.
func NormalizeProvider(p string) string { return strings.ToLower(strings.TrimSpace(p)) }
