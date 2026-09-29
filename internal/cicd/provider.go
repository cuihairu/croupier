// Package cicd 定义外部 CI/CD 系统的可插拔 provider 抽象（OPEN-ISSUES
// #58）：触发构建 / 查询构建状态 / 列产物 / webhook 状态回写。Jenkins、
// GitLab CI、GitHub Actions 与 generic（自定义 REST）均为注册的 provider
// 之一——不允许任何单一系统的协议细节渗出到接口之外。
//
// 分层：本包只有接口 + 注册表；各 provider 实现放 internal/cicd/providers
// （经 init 自注册）；消费方 import providers 包完成注册后经 New 构造。
package cicd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// 构建状态闭集（与 model.NormalizeCicdBuildStatus 一致；独立常量避免
// cicd → model 的 import 方向，webhook 归一仍以 model 为准）。
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusUnknown   = "unknown"
)

// TriggerRequest 描述一次触发构建。
type TriggerRequest struct {
	// 流水线标识：jenkins job 名 / gitlab ref（分支或 tag）/ gh workflow
	// 文件名 / generic 透传
	Pipeline string
	// 参数（jenkins buildWithParameters / gh dispatch inputs / gitlab
	// variables / generic 并入触发 body）
	Params map[string]string
}

// BuildRef 是触发结果与查询目标（provider 侧构建标识）。
type BuildRef struct {
	Pipeline   string
	ExternalID string // jenkins 队列/构建 URL、gitlab pipeline id、gh run id、generic id
	WebURL     string
}

// BuildStatus 是一次构建的当前状态快照。
type BuildStatus struct {
	Pipeline         string
	ExternalID       string
	Status           string // 本包 Status* 闭集
	WebURL           string
	ArtifactURL      string
	ArtifactChecksum string
	StartedAt        *time.Time
	FinishedAt       *time.Time
}

// Artifact 是产物条目。
type Artifact struct {
	Name string
	Path string
	URL  string
	Size int64
}

// Provider 是单一外部 CI/CD 系统的适配接口。实现必须可并发使用。
type Provider interface {
	// Kind 返回注册类型（jenkins/gitlab-ci/github-actions/generic）。
	Kind() string
	// TriggerBuild 触发一次构建并返回 provider 侧引用。
	TriggerBuild(ctx context.Context, req TriggerRequest) (*BuildRef, error)
	// GetBuild 拉取构建状态。ExternalID 为空时 provider 自行决定默认目标
	// （如最近一次运行）并在结果里回填 ExternalID。
	GetBuild(ctx context.Context, ref BuildRef) (*BuildStatus, error)
	// ListArtifacts 列出构建产物；无产物或 provider 不支持时返回空切片。
	ListArtifacts(ctx context.Context, ref BuildRef) ([]Artifact, error)
}

// Config 是 provider 构造配置（来自 CicdIntegration 行；API 层负责掩码）。
type Config struct {
	Endpoint string            // base URL（http/https）
	Token    string            // 凭据（可为空——部分自建系统匿名可读）
	Extra    map[string]string // provider 专属键（见各实现文档）
	// HTTP 注入测试用客户端；nil 时由消费方构造（生产路径应传
	// secguard.HTTPClient 守卫后的客户端）。
	HTTP *http.Client
	// Now 注入时钟（测试用）；nil 用 time.Now。
	Now func() time.Time
}

// Factory 由 Config 构造 provider；配置不合法（缺必填键/URL 形态错）返回错误。
type Factory func(cfg Config) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// ErrUnknownKind 是 New 查不到注册类型时的错误。
var ErrUnknownKind = errors.New("unknown cicd provider kind")

// Register 注册 provider 工厂；重复注册同名类型视为编程错误，panic fail-fast。
func Register(kind string, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if kind == "" || f == nil {
		panic("cicd: Register requires kind and factory")
	}
	if _, dup := registry[kind]; dup {
		panic("cicd: provider registered twice: " + kind)
	}
	registry[kind] = f
}

// New 按类型构造 provider。
func New(kind string, cfg Config) (Provider, error) {
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
