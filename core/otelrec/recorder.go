// Package otelrec 定义 agent 通用 OTel 观测面（todo.md OBS-AGENT-001）。
//
// agents/* 生产代码不得 import internal/（core/README.md 契约），因此把
// 观测 recorder 的接口面放在 core：四类 agent 只依赖接口与 noop 默认，
// 真实 OTel 装配由 cmd/* 入口经 internal/telemetry 适配后注入。
//
// 总开关语义与 server 一致：telemetry.enabled=false 时不建任何 exporter /
// 周期导出任务，调用点走 NoopRecorder（零分配直通）；enabled=true 且
// no_otel 裁剪构建时，cmd 入口 fail-closed 报错（不静默无导出）。
package otelrec

import "context"

// Recorder 是 agent 观测点的最小面。实现必须并发安全；NoopRecorder
// 满足零开销路径。各 agent 在关键事件点调用（启停/连接状态/吞吐/错误）。
type Recorder interface {
	// Start 标记 agent 生命周期开始（一次进程一次）。
	Start(ctx context.Context, attrs map[string]string)
	// Stop 标记 agent 生命周期结束。
	Stop(ctx context.Context, err error)
	// Connected 上游控制通道注册成功（重连可重复触发）。
	Connected(ctx context.Context)
	// Disconnected 上游控制通道断开/注册失败。
	Disconnected(ctx context.Context, err error)
	// Count 通用事件计数（agent 名 + 事件名 + 维度）。
	Count(ctx context.Context, event string, n int64, attrs map[string]string)
	// Observe 通用观测值（如探测延迟、吞吐计数）。
	Observe(ctx context.Context, event string, value float64, attrs map[string]string)
}

// NoopRecorder 总关闭/未装配时的零开销实现。
type NoopRecorder struct{}

func (NoopRecorder) Start(context.Context, map[string]string)                    {}
func (NoopRecorder) Stop(context.Context, error)                                 {}
func (NoopRecorder) Connected(context.Context)                                   {}
func (NoopRecorder) Disconnected(context.Context, error)                         {}
func (NoopRecorder) Count(context.Context, string, int64, map[string]string)     {}
func (NoopRecorder) Observe(context.Context, string, float64, map[string]string) {}

// Discard 返回共享 noop 实例（避免每 agent 分配）。
func Discard() Recorder { return NoopRecorder{} }
