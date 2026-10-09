// Package configsync 配置拉取与热生效的通用件（agent-core K4，泛化自
// sidecar-agent 的 extension sync puller）：「版本号轮询 → 拉取 → 热生效」。
// 业务方注入 Fetch（HTTP/gRPC/文件，含 wire 解码）与 Apply（热生效动作），
// 本包只管轮询节奏、版本跳过与错误回调。首样例：extension sync puller
// 切换；capture 白名单/阈值、devops 规则配置复用同一通道。
package configsync

import (
	"context"
	"sync"
	"time"
)

// DefaultInterval 轮询默认档（与 extension puller 先例一致）。
const DefaultInterval = 30 * time.Second

// Versioned 一次拉取的版本化载荷。Version 为空串表示业务方无法判版本，
// 每次都 Apply；非空且与上次生效版本相同则跳过（幂等热更的省叶轮）。
type Versioned[T any] struct {
	Version string
	Payload T
}

// FetchFunc 拉取一份版本化载荷（wire 契约与解码属业务方）。
type FetchFunc[T any] func(ctx context.Context) (*Versioned[T], error)

// ApplyFunc 热生效。返回错误时版本游标不推进，下个轮询重试。
type ApplyFunc[T any] func(ctx context.Context, payload T) error

// Puller 版本化配置轮询器。
type Puller[T any] struct {
	interval time.Duration
	fetch    FetchFunc[T]
	apply    ApplyFunc[T]
	onError  func(error)          // 拉取/生效失败回调（告警接入点，herald 消费）
	onApply  func(version string) // 热生效回调（指标/日志打点）

	mu          sync.Mutex
	lastVersion string
}

// Option 装配可选项。
type Option[T any] func(*Puller[T])

// WithOnError 设置失败回调。
func WithOnError[T any](fn func(error)) Option[T] {
	return func(p *Puller[T]) { p.onError = fn }
}

// WithOnApply 设置热生效回调（入参为生效的版本号）。
func WithOnApply[T any](fn func(version string)) Option[T] {
	return func(p *Puller[T]) { p.onApply = fn }
}

// New 装配轮询器。interval<=0 取 DefaultInterval；fetch/apply 必填，
// 缺一时 PullOnce 报错、Start 空转不 panic。
func New[T any](interval time.Duration, fetch FetchFunc[T], apply ApplyFunc[T], opts ...Option[T]) *Puller[T] {
	if interval <= 0 {
		interval = DefaultInterval
	}
	p := &Puller[T]{interval: interval, fetch: fetch, apply: apply}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// LastVersion 最近一次生效的版本号（未生效过为空串）。
func (p *Puller[T]) LastVersion() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastVersion
}
