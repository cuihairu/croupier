// Package provider 第三方平台集成的历史抽象（#66 P0 归档态）。
//
// 本包的 Registry 已删除（与 extension runtime bindings 构成双注册中心，
// 裁决见 docs/design/provider-plugin-design.md §5.3）：runtime 真值在
// extension bindings，driver 层（internal/drivers）是接口的新家。
//
// 保留物与去向：
//   - Provider 接口保留为 **driver 契约蓝本**——Call(ctx, method, []byte)
//     ([]byte, error) 的 JSON 透传语义正是 drivers.Driver.Call 的原型；
//     internal/platform/quicksdk、internal/platform/openapi 两个实现随
//     P1 迁移（mapping §2.3），迁完本包删除。
//   - ProviderNotFoundError / MethodNotSupportedError / ProviderDisabledError
//     三个错误类型语义由 drivers 层沿用（drivers.NotFoundError 等）。
package provider

import (
	"context"
	"fmt"
)

// Provider defines the interface that all third-party platform integrations must implement.
//
// Deprecated: 契约蓝本（#66 P0）。新代码使用 internal/drivers.Driver；
// 本接口仅为 P1 迁移前的 quicksdk/openapi 实现保留。
//
// A Provider represents a third-party service (like QuickSDK, ThinkingData, etc.)
// that can be called through the Croupier platform.
type Provider interface {
	// Name returns the unique identifier for this provider (e.g., "quicksdk", "thinkingdata").
	Name() string

	// Init initializes the provider with the given configuration.
	// This is called once when the provider is registered.
	Init(ctx context.Context, config ProviderConfig) error

	// IsEnabled returns whether this provider is currently enabled.
	IsEnabled() bool

	// SupportedMethods returns a list of method names that this provider supports.
	// For example: ["day_report", "user_live", "order_list"]
	SupportedMethods() []string

	// Call invokes a method on the provider with the given request and response.
	// The request and response are JSON encoded/decoded bytes.
	Call(ctx context.Context, method string, request []byte) ([]byte, error)

	// Close gracefully shuts down the provider and releases any resources.
	Close() error
}

// ProviderConfig holds the configuration for a provider.
type ProviderConfig struct {
	// Enabled determines whether the provider is active.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Type is the provider type identifier (e.g., "quicksdk").
	Type string `yaml:"type" json:"type"`

	// Config holds provider-specific configuration as a map.
	// Each provider is responsible for validating and extracting its own config.
	Config map[string]interface{} `yaml:"config" json:"config"`

	// RateLimit configures rate limiting for this provider.
	RateLimit *RateLimitConfig `yaml:"rate_limit" json:"rateLimit"`
}

// RateLimitConfig defines rate limiting parameters.
type RateLimitConfig struct {
	// RequestsPerMinute is the maximum number of requests allowed per minute.
	RequestsPerMinute int `yaml:"requests_per_minute" json:"requestsPerMinute"`

	// BurstSize allows temporary bursts above the sustained rate.
	BurstSize int `yaml:"burst_size" json:"burstSize"`
}

// ProviderNotFoundError is returned when a provider is not found in the registry.
type ProviderNotFoundError struct {
	Name string
}

func (e *ProviderNotFoundError) Error() string {
	return fmt.Sprintf("provider not found: %s", e.Name)
}

// MethodNotSupportedError is returned when a method is not supported by the provider.
type MethodNotSupportedError struct {
	Provider string
	Method   string
}

func (e *MethodNotSupportedError) Error() string {
	return fmt.Sprintf("method %q not supported by provider %q", e.Method, e.Provider)
}

// ProviderDisabledError is returned when attempting to call a disabled provider.
type ProviderDisabledError struct {
	Name string
}

func (e *ProviderDisabledError) Error() string {
	return fmt.Sprintf("provider %q is disabled", e.Name)
}
