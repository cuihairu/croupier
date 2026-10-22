package vault

import (
	"fmt"
	"sync"
	"time"
)

// Provider 键闭集（注册表键；与配置 vault.provider 对应）。
const (
	// ProviderLocal 本地加密文件存储（AES-256-GCM + scrypt KDF）。
	ProviderLocal = "local"
	// ProviderEnv 环境变量源（只读；路径映射到带前缀的环境变量名）。
	ProviderEnv = "env"
)

// SourceOptions 提供商构造参数（各实现按需取用）。与配置段
// vault.providers.<key> 一一对应，svc 装配层从 config 映射而来。
type SourceOptions struct {
	// Path local：加密存储文件路径（空取实现默认）。
	Path string
	// PassphraseEnv local：持主口令的环境变量名（空取实现默认；
	// 口令本身不落配置文件——herald tokenEnv 同款口径）。
	PassphraseEnv string
	// Prefix env：环境变量名前缀（空取实现默认）。
	Prefix string
	// SessionTTL 解锁会话空闲超时（local；<=0 取实现默认）。
	SessionTTL time.Duration
	// Now 时钟（单测注入；nil 取 time.Now）。
	Now func() time.Time
}

// Factory 提供商工厂（编译期注册，配置选择实现）。
type Factory func(opts SourceOptions) (Provider, error)

// 编译期工厂注册表（serverstatus/outlet 同构）：新增提供商=注册一项，
// 业务代码零改动，切换=换配置注册项。
var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

func init() {
	RegisterFactory(ProviderLocal, func(opts SourceOptions) (Provider, error) { return NewLocalProvider(opts) })
	RegisterFactory(ProviderEnv, func(opts SourceOptions) (Provider, error) { return NewEnvProvider(opts), nil })
}

// RegisterFactory 注册提供商工厂（同名覆盖，测试可注入替身）。
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

// Build 按注册表键构造提供商；未注册键报错（点名已知键）。
func Build(name string, opts SourceOptions) (Provider, error) {
	factory := Lookup(name)
	if factory == nil {
		return nil, fmt.Errorf("vault provider %q not registered (known: %v)", name, Names())
	}
	return factory(opts)
}

// Names 返回已注册键（诊断/文档用，顺序不保证）。
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	return names
}
