package serverstatus

import "context"

// NoopProvider 真实 no-op：恒返回「无维护信息」（InMaintenance=false，
// Source=noop，无错误）。零配置=零维护信息、gate 直通——全部照报，行为与
// 未接状态源一致。no-op 是真实注册实现，不是 if 散落（插件机制决策①）。
type NoopProvider struct{}

// NewNoopProvider 创建 no-op 状态源。
func NewNoopProvider() *NoopProvider { return &NoopProvider{} }

// Name 注册表键。
func (p *NoopProvider) Name() string { return ProviderNoop }

// GetServerStatus 恒定无维护信息。
func (p *NoopProvider) GetServerStatus(_ context.Context, _ ServerRef) (ServerStatus, error) {
	return ServerStatus{Source: ProviderNoop}, nil
}

// ListServers no-op 无服务器清单。
func (p *NoopProvider) ListServers(_ context.Context) ([]ServerSummary, error) {
	return []ServerSummary{}, nil
}
