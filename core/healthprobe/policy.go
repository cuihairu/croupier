package healthprobe

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// 采样策略（评审简档 §13.3 C）：频率是 server 下发的策略，agent 不本地写死。
// JSON 形态（config_versions 键 probe.policy，namespace=ops；agent 经
// configsync 下拉热更，面板与 agent 两侧共用本包解析校验）：
//
//	{
//	  "interval": "30s",   // ① 基础采样频率（缺省 30s）
//	  "autoBoost": true,   // 异常态提频开关（缺省开）
//	  "targets": [ { "target": "db-primary", "interval": "10s" } ],
//	  "windows": [ { "start": "19:00", "end": "23:00", "interval": "10s" } ],
//	  "events":  [ { "at": "2026-10-20T09:00:00+08:00", "duration": "30m", "interval": "5s" } ]
//	}
//
// 叠加优先级（C#2「未设置=继承基础间隔，无隐藏层级」）：特殊时间点 event >
// 时间段 window > 按目标 target > 基础 interval。时段用服务器本地时区钟点
// （不跨午夜，跨段拆两段声明）；event 用 RFC3339 带显式偏移（无时区歧义）。
// 时段降频只作用于成功样本的节奏；目标处于开窗（unavailable）时异常态提频
// 覆盖时段策略（C#5），见 EffectiveIntervalFor。

// PolicyBoostFloor 异常态提频下限（C#5：interval/2、下限 5s 防内网打爆）。
const PolicyBoostFloor = 5 * time.Second

// ProbePolicy probe.policy 顶层策略（区间/时点粒度全显式，未设置即继承）。
type ProbePolicy struct {
	Interval  string              `json:"interval,omitempty"`
	AutoBoost *bool               `json:"autoBoost,omitempty"`
	Targets   []ProbePolicyTarget `json:"targets,omitempty"`
	Windows   []ProbePolicyWindow `json:"windows,omitempty"`
	Events    []ProbePolicyEvent  `json:"events,omitempty"`
}

// ProbePolicyTarget 按目标覆盖采样频率（target 名须与 probe.targets 声明一致）。
type ProbePolicyTarget struct {
	Target   string `json:"target"`
	Interval string `json:"interval"`
}

// ProbePolicyWindow 按时间段覆盖（服务器本地时区钟点，start<end 同日内）。
type ProbePolicyWindow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Interval string `json:"interval"`
}

// ProbePolicyEvent 特殊时间点（一次性；RFC3339 带显式偏移，如活动开服前）。
type ProbePolicyEvent struct {
	At       string `json:"at"`
	Duration string `json:"duration"`
	Interval string `json:"interval"`
}

// ParseProbePolicy 解析并校验 probe.policy JSON（面板发布预检与 agent apply
// 同一校验真值，capture gates 先例）。空内容拒绝——「未发布」语义由调用方
// （空版本不下发）承担，不靠空串静默通过。
func ParseProbePolicy(content string) (*ProbePolicy, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("probe policy content is empty")
	}
	var p ProbePolicy
	if err := json.Unmarshal([]byte(content), &p); err != nil {
		return nil, fmt.Errorf("probe policy is not a valid object: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate 语义校验：时长全部可解析且为正；时段钟点合法且 start<end（不跨
// 午夜）；event 为 RFC3339 带偏移且时长为正；目标名非空且不重复。
func (p *ProbePolicy) Validate() error {
	if p == nil {
		return nil
	}
	if p.Interval != "" {
		if _, err := parsePolicyDuration(p.Interval); err != nil {
			return fmt.Errorf("probe policy interval: %w", err)
		}
	}
	seen := make(map[string]bool, len(p.Targets))
	for _, t := range p.Targets {
		if strings.TrimSpace(t.Target) == "" {
			return fmt.Errorf("probe policy targets: target name is required")
		}
		if seen[t.Target] {
			return fmt.Errorf("probe policy targets: duplicate target %q", t.Target)
		}
		seen[t.Target] = true
		if _, err := parsePolicyDuration(t.Interval); err != nil {
			return fmt.Errorf("probe policy targets[%s] interval: %w", t.Target, err)
		}
	}
	for i, w := range p.Windows {
		sh, sm, err := parseClock(w.Start)
		if err != nil {
			return fmt.Errorf("probe policy windows[%d] start: %w", i, err)
		}
		eh, em, err := parseClock(w.End)
		if err != nil {
			return fmt.Errorf("probe policy windows[%d] end: %w", i, err)
		}
		if eh*60+em <= sh*60+sm {
			return fmt.Errorf("probe policy windows[%d]: end must be after start (same day; split overnight windows)", i)
		}
		if _, err := parsePolicyDuration(w.Interval); err != nil {
			return fmt.Errorf("probe policy windows[%d] interval: %w", i, err)
		}
	}
	for i, e := range p.Events {
		if _, err := time.Parse(time.RFC3339, e.At); err != nil {
			return fmt.Errorf("probe policy events[%d] at: must be RFC3339 with explicit offset: %w", i, err)
		}
		if _, err := parsePolicyDuration(e.Duration); err != nil {
			return fmt.Errorf("probe policy events[%d] duration: %w", i, err)
		}
		if _, err := parsePolicyDuration(e.Interval); err != nil {
			return fmt.Errorf("probe policy events[%d] interval: %w", i, err)
		}
	}
	return nil
}

// BaseInterval 基础采样频率（未设置或解析异常回 DefaultInterval——解析异常
// 已被 ParseProbePolicy 拦在门外，此处仅防御内存构造）。
func (p *ProbePolicy) BaseInterval() time.Duration {
	if p == nil {
		return DefaultInterval
	}
	if d, err := parsePolicyDuration(p.Interval); err == nil {
		return d
	}
	return DefaultInterval
}

// AutoBoostEnabled 异常态提频开关（缺省开）。
func (p *ProbePolicy) AutoBoostEnabled() bool {
	return p == nil || p.AutoBoost == nil || *p.AutoBoost
}

// EffectiveInterval 目标 t 在 now 的生效采样频率（不含异常态提频）：
// event > window > target > base，未命中即继承下一层。
func (p *ProbePolicy) EffectiveInterval(target string, now time.Time) time.Duration {
	if p == nil {
		return DefaultInterval
	}
	for _, e := range p.Events {
		at, err := time.Parse(time.RFC3339, e.At)
		if err != nil {
			continue
		}
		dur, err := parsePolicyDuration(e.Duration)
		if err != nil {
			continue
		}
		if !now.Before(at) && now.Before(at.Add(dur)) {
			return policyDurationOr(e.Interval, DefaultInterval)
		}
	}
	h, m := now.Hour(), now.Minute()
	for _, w := range p.Windows {
		sh, sm, err := parseClock(w.Start)
		if err != nil {
			continue
		}
		eh, em, err2 := parseClock(w.End)
		if err2 != nil {
			continue
		}
		if h*60+m >= sh*60+sm && h*60+m < eh*60+em {
			return policyDurationOr(w.Interval, DefaultInterval)
		}
	}
	for _, t := range p.Targets {
		if t.Target == target {
			return policyDurationOr(t.Interval, DefaultInterval)
		}
	}
	return p.BaseInterval()
}

// EffectiveIntervalFor 目标 t 在 now 的生效采样频率（含异常态提频，C#5）：
// 开窗（unavailable）且开关开 → interval/2、下限 5s、绝不慢于原值；覆盖
// 时段策略（提频分支直接给出结果，不与时段降频叠加）。
func (p *ProbePolicy) EffectiveIntervalFor(target string, now time.Time, unavailable bool) time.Duration {
	d := p.EffectiveInterval(target, now)
	if !unavailable || !p.AutoBoostEnabled() {
		return d
	}
	half := d / 2
	if half < PolicyBoostFloor {
		half = PolicyBoostFloor
	}
	if half > d {
		half = d
	}
	return half
}

// parsePolicyDuration 解析策略时长（"30s"/"10s"/"1m30s"；必须为正）。
func parsePolicyDuration(s string) (time.Duration, error) {
	d, err := time.ParseDuration(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration %q must be positive", s)
	}
	return d, nil
}

// parseClock 解析 "15:04" 钟点（返回时:分）。
func parseClock(s string) (hour, minute int, err error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid clock time %q (expect HH:MM)", s)
	}
	return t.Hour(), t.Minute(), nil
}

func policyDurationOr(s string, fallback time.Duration) time.Duration {
	if d, err := parsePolicyDuration(s); err == nil {
		return d
	}
	return fallback
}
