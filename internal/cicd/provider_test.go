package cicd_test

// provider 抽象契约测试（OPEN-ISSUES #58）：注册表行为 + 全部内置 provider
// 满足接口、类型闭集与 model 层枚举对齐。

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/cicd"
	_ "github.com/cuihairu/croupier/internal/cicd/providers"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistry_Contract 注册表：内置四类型已注册、未知类型报 ErrUnknownKind、
// Kinds 升序稳定。
func TestRegistry_Contract(t *testing.T) {
	kinds := cicd.Kinds()
	assert.Equal(t, []string{"generic", "github-actions", "gitlab-ci", "jenkins"}, kinds)

	_, err := cicd.New("teamcity", cicd.Config{})
	require.ErrorIs(t, err, cicd.ErrUnknownKind)
}

// TestRegistry_AllKindsImplementProvider 每个注册类型都能以合法最小配置
// 构造（generic 需要至少一个 URL；github/gitlab/jenkins 只需 endpoint），
// 且满足 Provider 接口。
func TestRegistry_AllKindsImplementProvider(t *testing.T) {
	minCfg := map[string]cicd.Config{
		"jenkins": {Endpoint: "https://ci.example.com"},
		"gitlab-ci": {Endpoint: "https://gitlab.example.com",
			Extra: map[string]string{"project": "1"}},
		"github-actions": {Endpoint: "https://api.github.com",
			Extra: map[string]string{"repo": "o/r"}},
		"generic": {Endpoint: "https://ci.example.com",
			Extra: map[string]string{"statusUrl": "https://ci.example.com/status/{id}"}},
	}
	for _, kind := range cicd.Kinds() {
		cfg, ok := minCfg[kind]
		require.True(t, ok, "kind %s 缺最小配置用例", kind)
		p, err := cicd.New(kind, cfg)
		require.NoError(t, err, "kind=%s", kind)
		var iface cicd.Provider = p
		assert.Equal(t, kind, iface.Kind())
		// 接口方法可调用（错误路径也行——这里传背景 ctx 无网络，期待非 panic）
		_, _ = iface.GetBuild(context.Background(), cicd.BuildRef{})
	}
}

// TestRegister_DuplicatePanics 重复注册同名类型 panic（fail-fast）。
func TestRegister_DuplicatePanics(t *testing.T) {
	assert.Panics(t, func() {
		cicd.Register("jenkins", func(cicd.Config) (cicd.Provider, error) {
			return nil, errors.New("dup")
		})
	})
}

// TestStatusNormalization_AlignedToModel cicd 状态闭集与 model 归一结果
// 语义对齐（webhook 走 model，provider 拉取走各实现映射，两侧不得分叉）。
func TestStatusNormalization_AlignedToModel(t *testing.T) {
	pairs := map[string]string{
		"jenkins SUCCESS": model.CicdBuildSuccess,
	}
	assert.Equal(t, pairs["jenkins SUCCESS"], model.NormalizeCicdBuildStatus("SUCCESS"))
	for _, s := range []string{model.CicdBuildQueued, model.CicdBuildRunning,
		model.CicdBuildSuccess, model.CicdBuildFailed,
		model.CicdBuildCancelled, model.CicdBuildUnknown} {
		assert.Equal(t, s, model.NormalizeCicdBuildStatus(s))
	}
}
