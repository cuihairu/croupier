package serverstatus

import (
	"sync"
	"time"
)

// Provider 键闭集（注册表键；与配置 serverStatus.provider 对应）。
const (
	ProviderNoop  = "noop"
	ProviderAtlas = "atlas"
)

// Options 状态源构造参数（各实现按需取用；no-op 忽略）。与配置段
// serverStatus.providers.<key> 一一对应，svc 装配层从 config 映射而来。
type Options struct {
	// BaseURL 状态源服务地址（atlas 等真实源必填）。
	BaseURL string
	// Token 凭据明文（svc 装配层从 tokenEnv 环境变量解析后注入；
	// 状态源包内不读环境变量，便于单测）。
	Token string
	// Timeout 单次查询超时；<=0 取实现默认。
	Timeout time.Duration
	// Atlas atlas 适配器专属（路径模板 + 字段映射；其他实现忽略）。
	Atlas AtlasOptions
}

// Factory 状态源工厂（编译期注册，配置选择实现）。
type Factory func(opts Options) Provider

// 编译期工厂注册表（与 Outlet 同构）：新增状态源=注册一项，gate 业务代码
// 零改动，切换=换配置注册项。
var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

func init() {
	RegisterFactory(ProviderNoop, func(Options) Provider { return NewNoopProvider() })
	RegisterFactory(ProviderAtlas, func(opts Options) Provider { return NewAtlasProvider(opts) })
}

// RegisterFactory 注册状态源工厂（同名覆盖，测试可注入替身）。
func RegisterFactory(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

// Lookup 按键取工厂；未注册键返回 nil。
func Lookup(name string) Factory {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[name]
}

// Names 返回已注册键（设置页下拉/诊断用，顺序不保证）。
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	return names
}
