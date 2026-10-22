// Package serverstatus 提供服务器维护状态 Provider 抽象（维护窗口 gate，
// 不绑死 atlas）。状态源 Provider、告警出口 Provider、供应商 Provider 三者
// 同构（plugin-mechanism 设计决策①/⑦）：编译期注册表 + 配置选择实现 +
// 默认 no-op 第一原则。
//
// 判定规则（需求令原文）：agent 探活异常 ∧ 状态源不在维护窗 → 报警；
// 状态源不可达=按未知处理，照报警并标 source_unknown，不许静默。
package serverstatus

import (
	"context"
	"errors"
	"time"
)

// Provider 服务器维护状态源。默认 no-op（无维护信息，gate 直通）；atlas 是
// 第一个真实选项，Zabbix/Prometheus/云监控按同接口实现。
type Provider interface {
	// Name 返回注册表键（"noop" / "atlas" / …）。
	Name() string
	// GetServerStatus 查询目标服务器维护状态。错误=状态未知（gate 按
	// fail-open 处理：照报并标 source_unknown，绝不静默）。
	GetServerStatus(ctx context.Context, ref ServerRef) (ServerStatus, error)
	// ListServers 返回状态源已知的服务器清单（设置页映射校验/面板用）。
	// 不用于 gate 判定；实现可返回空切片。
	ListServers(ctx context.Context) ([]ServerSummary, error)
}

// ServerRef 目标服务器定位（gate 侧从事件解析后传入）。匹配键优先级见
// Gate（matchBy 决定 agentId / host / ip）。
type ServerRef struct {
	AgentID string
	Host    string
	IP      string
	GameID  string
	Env     string
}

// Valid 判断定位是否足以查询状态源（无任何可用键=不过 gate 照常投递）。
func (r ServerRef) Valid() bool {
	return r.AgentID != "" || r.Host != "" || r.IP != ""
}

// CacheKey 是缓存用的稳定键（定位语义上 agentId/host/ip 互为备选，
// gate 侧已按 matchBy 归一到首选键）。
func (r ServerRef) CacheKey() string {
	switch {
	case r.AgentID != "":
		return "agent:" + r.AgentID
	case r.Host != "":
		return "host:" + r.Host
	default:
		return "ip:" + r.IP
	}
}

// MaintenanceWindow 当前/最近维护窗口。
type MaintenanceWindow struct {
	Start time.Time
	End   time.Time
	Note  string
}

// ServerStatus 统一契约（需求令原文 getServerStatus(server) ->
// {healthy, in_maintenance, window, source}）。InMaintenance 是 gate 唯一
// 裁决字段；Healthy 是状态源自视角的服务器健康（面板展示用，不参与裁决）。
type ServerStatus struct {
	Healthy       bool
	InMaintenance bool
	Window        *MaintenanceWindow
	Source        string
}

// ServerSummary 状态源已知服务器（ListServers 条目）。
type ServerSummary struct {
	AgentID       string
	Host          string
	Healthy       bool
	InMaintenance bool
}

// 错误闭集（gate 侧 errors.Is 判定，不用字符串匹配）。
var (
	// ErrNoMapping 状态源没有该服务器的映射记录（未配置映射→未知）。
	ErrNoMapping = errors.New("server status: no mapping for server ref")
	// ErrProviderUnavailable 状态源不可达/查询失败（未知→照报标注）。
	ErrProviderUnavailable = errors.New("server status: provider unavailable")
)
