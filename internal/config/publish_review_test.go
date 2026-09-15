package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// T10 发布分级：ResolvePublishReview 优先级与从严语义矩阵。
// ByEnv[env] > 全局 publishReview > 内置默认（dev=auto，其余=required）；
// 空 env 与非法值一律从严 required。
func TestPagesConfig_ResolvePublishReview(t *testing.T) {
	cases := []struct {
		name string
		cfg  PagesConfig
		env  string
		want string
	}{
		{"内置默认 dev=auto", PagesConfig{}, "dev", PublishReviewAuto},
		{"内置默认 prod=required", PagesConfig{}, "prod", PublishReviewRequired},
		{"内置默认未知 env=required", PagesConfig{}, "staging", PublishReviewRequired},
		{"空 env 从严 required", PagesConfig{}, "  ", PublishReviewRequired},
		{"全局 auto 覆盖默认", PagesConfig{PublishReview: PublishReviewAuto}, "prod", PublishReviewAuto},
		{"全局 required 覆盖 dev 默认", PagesConfig{PublishReview: PublishReviewRequired}, "dev", PublishReviewRequired},
		{"ByEnv 覆盖全局", PagesConfig{
			PublishReview:      PublishReviewRequired,
			PublishReviewByEnv: map[string]string{"dev": PublishReviewAuto},
		}, "dev", PublishReviewAuto},
		{"ByEnv 未覆盖的 env 落回全局", PagesConfig{
			PublishReview:      PublishReviewAuto,
			PublishReviewByEnv: map[string]string{"dev": PublishReviewRequired},
		}, "staging", PublishReviewAuto},
		{"非法全局值从内置默认", PagesConfig{PublishReview: "sometimes"}, "dev", PublishReviewAuto},
		{"非法全局值非 dev 从严", PagesConfig{PublishReview: "sometimes"}, "prod", PublishReviewRequired},
		{"非法 ByEnv 值落回全局", PagesConfig{
			PublishReview:      PublishReviewRequired,
			PublishReviewByEnv: map[string]string{"dev": "whenever"},
		}, "dev", PublishReviewRequired},
		{"env 前后空白被规整后命中 ByEnv", PagesConfig{
			PublishReviewByEnv: map[string]string{"dev": PublishReviewAuto},
		}, " dev ", PublishReviewAuto},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cfg.ResolvePublishReview(tc.env))
		})
	}
}

func TestPagesConfig_ValidPublishReview(t *testing.T) {
	assert.True(t, validPublishReview(PublishReviewAuto))
	assert.True(t, validPublishReview(PublishReviewRequired))
	assert.False(t, validPublishReview(""))
	assert.False(t, validPublishReview("AUTO"))
}
