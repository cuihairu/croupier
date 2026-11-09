package config

import (
	"strings"
)

// PagesConfig 页面发布分级配置。
type PagesConfig struct {
	// PublishReview 全局策略：auto | required。为空时按 env 内置默认
	// （dev→auto，其余 env→required）。
	PublishReview string `json:"publishReview,omitempty" yaml:"publishReview,omitempty"`
	// PublishReviewByEnv env 级覆盖（优先于 PublishReview 与内置默认），
	// 如 {dev: required, staging: auto}。
	PublishReviewByEnv map[string]string `json:"publishReviewByEnv,omitempty" yaml:"publishReviewByEnv,omitempty"`
}

// ResolvePublishReview 解析 env 的发布策略（T10/D5）。
//
// 优先级：publishReviewByEnv[env] > publishReview > 内置默认（dev→auto，
// 其余→required）。缺 env（请求未携带 X-Env）与非法取值一律从严 required。
func (c PagesConfig) ResolvePublishReview(env string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		return PublishReviewRequired
	}
	if v, ok := c.PublishReviewByEnv[env]; ok && validPublishReview(v) {
		return v
	}
	if validPublishReview(c.PublishReview) {
		return c.PublishReview
	}
	if env == "dev" {
		return PublishReviewAuto
	}
	return PublishReviewRequired
}

func validPublishReview(v string) bool {
	return v == PublishReviewAuto || v == PublishReviewRequired
}

// DefaultPipelineOperationGuard 大文档提示默认阈值（M6）。
const DefaultPipelineOperationGuard = 500

// OpenAPIConfig OpenAPI 上传管线配置。
type OpenAPIConfig struct {
	// PipelineOperationGuard 大文档提示阈值：operations 数超过该值的上传
	// 在管线摘要里追加 warn 级 large_document_pipeline diagnostic（建议
	// 分批/拆分 source，同步管线有超时风险），不阻断上传。0=默认 500，
	// 负数=禁用。
	PipelineOperationGuard int `json:"pipelineOperationGuard,omitempty" yaml:"pipelineOperationGuard,omitempty"`
}

// PipelineOperationGuardThreshold 解析生效阈值。返回 (0, false) 表示护栏
// 禁用；0 值配置回落 DefaultPipelineOperationGuard。
func (c OpenAPIConfig) PipelineOperationGuardThreshold() (int, bool) {
	if c.PipelineOperationGuard < 0 {
		return 0, false
	}
	if c.PipelineOperationGuard == 0 {
		return DefaultPipelineOperationGuard, true
	}
	return c.PipelineOperationGuard, true
}
