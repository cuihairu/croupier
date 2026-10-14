package healthprobe

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// DefaultInterval 默认采样频率（30s）：与 configsync 轮询、metrics 上报同档；
// 配内核默认 fail=3 → 最坏 90s 开窗（健康评审 §13.3 显式默认值）。
const DefaultInterval = 30 * time.Second

// 目标声明类型闭集。
const (
	TargetTypeHTTP = "http"
	TargetTypeTCP  = "tcp"
	TargetTypeDB   = "db"
)

// TargetConfig 探针目标声明（agent 配置 probe.targets[] 元素）：只声明探什么；
// 语义/失败阈值/故障窗口/打点一律复用本包（一个能力一份实现，核心零外部依赖）。
// 配置键 lowerCamelCase（http/tcp 域内；db 域用 dsn 等本包契约键）。
type TargetConfig struct {
	Name             string        `json:"name" yaml:"name"`
	Type             string        `json:"type" yaml:"type"` // http | tcp | db
	URL              string        `json:"url,omitempty" yaml:"url,omitempty"`
	Address          string        `json:"address,omitempty" yaml:"address,omitempty"`
	DSN              string        `json:"dsn,omitempty" yaml:"dsn,omitempty"`
	Driver           string        `json:"driver,omitempty" yaml:"driver,omitempty"`
	Query            string        `json:"query,omitempty" yaml:"query,omitempty"`
	Timeout          time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Semantics        Semantics     `json:"semantics,omitempty" yaml:"semantics,omitempty"`
	Interval         time.Duration `json:"interval,omitempty" yaml:"interval,omitempty"`
	FailThreshold    int           `json:"failThreshold,omitempty" yaml:"failThreshold,omitempty"`
	RecoverThreshold int           `json:"recoverThreshold,omitempty" yaml:"recoverThreshold,omitempty"`
}

// EffectiveInterval 返回生效采样频率：显式值优先，否则全局默认 30s。
func (t TargetConfig) EffectiveInterval() time.Duration {
	if t.Interval > 0 {
		return t.Interval
	}
	return DefaultInterval
}

// NewChecker 按目标声明构造 Checker（工厂收口在本包——各 agent 不许各写探测装配）。
func NewChecker(t TargetConfig) (Checker, error) {
	switch strings.ToLower(t.Type) {
	case TargetTypeHTTP:
		if strings.TrimSpace(t.URL) == "" {
			return nil, fmt.Errorf("target %q: http requires url", t.Name)
		}
		if _, err := url.Parse(t.URL); err != nil {
			return nil, fmt.Errorf("target %q: invalid url: %w", t.Name, err)
		}
		to := t.Timeout
		if to <= 0 {
			to = 5 * time.Second
		}
		return NewHTTPChecker(t.URL, to), nil
	case TargetTypeTCP:
		if strings.TrimSpace(t.Address) == "" {
			return nil, fmt.Errorf("target %q: tcp requires address", t.Name)
		}
		to := t.Timeout
		if to <= 0 {
			to = 2 * time.Second
		}
		return NewTCPChecker(t.Address, to), nil
	case TargetTypeDB:
		if strings.TrimSpace(t.DSN) == "" || strings.TrimSpace(t.Driver) == "" {
			return nil, fmt.Errorf("target %q: db requires dsn and driver", t.Name)
		}
		return &DBChecker{DSN: t.DSN, Driver: t.Driver, Timeout: t.Timeout, Query: t.Query}, nil
	default:
		return nil, fmt.Errorf("target %q: unknown type %q (want http|tcp|db)", t.Name, t.Type)
	}
}

// ProbeFromTarget 从目标声明构造 Probe（阈值/语义/间隔取显式值或默认；
// onWindow/onResult 回调可选——窗口事件与失败明细的上行出口）。
// process 类型不进工厂：PID 由运行时（supervisor）解析，声明式配置无法表达。
func ProbeFromTarget(t TargetConfig, scopeGame, scopeEnv string, onWindow WindowListener, onResult ResultListener) (*Probe, error) {
	chk, err := NewChecker(t)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(t.Name) == "" {
		return nil, fmt.Errorf("target name is required")
	}
	sem := t.Semantics
	if sem == "" {
		sem = SemanticsReadiness
	}
	opts := []Option{WithScope(scopeGame, scopeEnv)}
	if onWindow != nil {
		opts = append(opts, OnWindow(onWindow))
	}
	if onResult != nil {
		opts = append(opts, OnResult(onResult))
	}
	if t.FailThreshold > 0 || t.RecoverThreshold > 0 {
		opts = append(opts, WithThresholds(t.FailThreshold, t.RecoverThreshold))
	}
	return New(t.Name, sem, chk, opts...), nil
}
