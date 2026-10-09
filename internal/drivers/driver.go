// Package drivers 外部平台接入的 driver 层（#66 Provider 插件批二 P0 契约定型）。
// 三层模型：extension = 可安装的插件包（catalog+release）；driver = 一种调用
// 协议的编译期内置实现（openapi/webhook 闭集）；provider = 一个 driver 的配
// 置实例（installation+runtime binding 承载）。runtime 真值在 extension
// bindings，本包只做编译期注册表——统一注册中心裁决见
// docs/design/provider-plugin-design.md §5.3。
//
// 接口语义以归档的 internal/platform/provider.Provider 为蓝本收窄
// （Call 的 JSON 透传语义即 driver Call 原型）：Kind/Call/Close，Init 并入
// 工厂（配置不合法在工厂返回错误），IsEnabled/SupportedMethods 由
// installation 状态与 binding spec 表达。注册表模式对齐 internal/cicd
// （init 自注册 + 重复注册 panic fail-fast）。
//
// P0 仅落契约：接口 + 空转注册表 + 契约错误，driver 实现随 P1 迁入
// （openapi driver 落本包 openapi 子目录）。
package drivers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Config 是 driver 构造配置（来自 provider binding spec 的 config 拆分；
// API 层负责掩码）。对齐 cicd.Config：Endpoint/Token/Extra + 可注入 HTTP
// 与 Now（测试用）。
type Config struct {
	Endpoint string            // base URL（http/https）
	Token    string            // 凭据（可为空——部分系统匿名可读）
	Extra    map[string]string // driver 专属键（见各实现文档）
	// HTTP 注入测试用客户端；nil 时由消费方构造（生产路径必须传
	// secguard 守卫后的客户端）。
	HTTP *http.Client
	// Now 注入时钟（测试用）；nil 用 time.Now。
	Now func() time.Time
}

// Driver 是单一外部系统调用协议的适配接口。实现必须可并发使用。
// Call 的 request/response 为 JSON 编码/解码字节（透传，driver 不解释语义）。
type Driver interface {
	// Kind 返回注册类型（"openapi" / "webhook"）。
	Kind() string
	// Call 调用 method 并返回 JSON 响应字节。
	Call(ctx context.Context, method string, request []byte) ([]byte, error)
	// Close 释放连接池等资源；幂等。
	Close() error
}

// Factory 由 Config 构造 driver 实例；配置不合法（缺必填键/URL 形态错）
// 返回错误（Init 并入构造，对齐 cicd Factory 先例）。
type Factory func(cfg Config) (Driver, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// ErrUnknownKind 是 New 查不到注册类型时的错误。
var ErrUnknownKind = errors.New("unknown driver kind")

// Register 注册 driver 工厂；重复注册同名类型视为编程错误，panic fail-fast。
// 各 driver 实现以 init 自注册（消费方 import 本包即完成注册）。
func Register(kind string, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if kind == "" || f == nil {
		panic("drivers: Register requires kind and factory")
	}
	if _, dup := registry[kind]; dup {
		panic("drivers: driver registered twice: " + kind)
	}
	registry[kind] = f
}

// New 按类型构造 driver 实例。
func New(kind string, cfg Config) (Driver, error) {
	registryMu.RLock()
	f, ok := registry[kind]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, kind)
	}
	return f(cfg)
}

// Kinds 返回已注册类型（升序稳定）。
func Kinds() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// 契约错误（语义沿用自归档的 internal/platform/provider，由本层承接）：
// NotFoundError = 运行时实例表未命中；MethodNotSupportedError = 方法不在该
// provider 的 operations 闭集；DisabledError = installation 已 disable
// （启停由 installation 状态表达，不经 driver 实现方维护）。

// NotFoundError 表示运行时按 provider 名查不到 driver 实例。
type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("provider not found: %s", e.Name)
}

// MethodNotSupportedError 表示 method 不在 provider 的 operations 闭集。
type MethodNotSupportedError struct {
	Provider string
	Method   string
}

func (e *MethodNotSupportedError) Error() string {
	return fmt.Sprintf("method %q not supported by provider %q", e.Method, e.Provider)
}

// DisabledError 表示目标 provider 的 installation 处于 disabled 状态。
type DisabledError struct {
	Name string
}

func (e *DisabledError) Error() string {
	return fmt.Sprintf("provider %q is disabled", e.Name)
}
