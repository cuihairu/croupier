package config

import (
	"strings"
	"time"
)

// ExecutionLogConfig 执行留痕（R1）：payload 级请求/响应落库 execution_logs。
type ExecutionLogConfig struct {
	// Enabled 默认开启；显式置 false 后同步执行不写留痕。
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// MaxPayloadBytes 单条请求/响应载荷上限，超出截断（默认 64KB）。
	MaxPayloadBytes int `json:"maxPayloadBytes,omitempty" yaml:"maxPayloadBytes,omitempty"`
	// RetentionDays 保留天数，未配置默认 7；显式置 0=永久保留（R3 清理循环消费）。
	RetentionDays *int `json:"retentionDays,omitempty" yaml:"retentionDays,omitempty"`
}

// IsEnabled 未配置时默认开启。
func (c ExecutionLogConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// EffectiveRetentionDays 未配置时默认 7 天；0=永久。
func (c ExecutionLogConfig) EffectiveRetentionDays() int {
	if c.RetentionDays == nil {
		return 7
	}
	return *c.RetentionDays
}

// TaskLogConfig 异步任务留痕（task_runs/task_events）保留策略。
type TaskLogConfig struct {
	// RetentionDays 保留天数，未配置默认 7；显式置 0=永久保留。
	RetentionDays *int `json:"retentionDays,omitempty" yaml:"retentionDays,omitempty"`
	// ReaperTimeout 无事件任务转 timed_out 的看门狗预算（duration 字符串，
	// 如 "5m"）。未配置/非法/≤0 默认 5m。ARCH-018 D3 看门狗收割器。
	ReaperTimeout string `json:"reaperTimeout,omitempty" yaml:"reaperTimeout,omitempty"`
}

// EffectiveRetentionDays 未配置时默认 7 天；0=永久。
func (c TaskLogConfig) EffectiveRetentionDays() int {
	if c.RetentionDays == nil {
		return 7
	}
	return *c.RetentionDays
}

// EffectiveReaperTimeout 未配置或解析失败时默认 5 分钟。
func (c TaskLogConfig) EffectiveReaperTimeout() time.Duration {
	if strings.TrimSpace(c.ReaperTimeout) == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(strings.TrimSpace(c.ReaperTimeout))
	if err != nil || d <= 0 {
		return 5 * time.Minute
	}
	return d
}

// ApprovalConfig 审批流行为配置。
type ApprovalConfig struct {
	// AllowSelfApprove 放开「申请人不能审批自己的申请」两人规则。
	// 默认 false；仅建议单管理员部署显式打开。
	AllowSelfApprove bool `json:"allowSelfApprove,omitempty" yaml:"allowSelfApprove,omitempty"`
}
