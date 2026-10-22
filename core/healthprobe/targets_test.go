package healthprobe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveInterval(t *testing.T) {
	assert.Equal(t, DefaultInterval, TargetConfig{
		Category: TargetCategoryGame, Name: "a"}.EffectiveInterval())
	assert.Equal(t, 10*time.Second, TargetConfig{
		Category: TargetCategoryGame, Name: "a", Interval: 10 * time.Second}.EffectiveInterval())
	// 负值视同未设置。
	assert.Equal(t, DefaultInterval, TargetConfig{
		Category: TargetCategoryGame, Name: "a", Interval: -1}.EffectiveInterval())
}

func TestNewCheckerHTTP(t *testing.T) {
	chk, err := NewChecker(TargetConfig{Name: "web", Category: TargetCategoryThirdParty, Type: TargetTypeHTTP, URL: "http://127.0.0.1:1/health"})
	require.NoError(t, err)
	h, ok := chk.(*HTTPChecker)
	require.True(t, ok)
	assert.Equal(t, 5*time.Second, h.Client.Timeout) // 默认 5s

	chk, err = NewChecker(TargetConfig{Name: "web", Category: TargetCategoryThirdParty, Type: TargetTypeHTTP, URL: "http://x", Timeout: 2 * time.Second})
	require.NoError(t, err)
	h = chk.(*HTTPChecker)
	assert.Equal(t, 2*time.Second, h.Client.Timeout)

	// 缺 url / 非法 url。
	_, err = NewChecker(TargetConfig{Name: "web", Category: TargetCategoryThirdParty, Type: TargetTypeHTTP})
	assert.ErrorContains(t, err, "requires url")
	_, err = NewChecker(TargetConfig{Name: "web", Category: TargetCategoryThirdParty, Type: TargetTypeHTTP, URL: "://bad"})
	assert.ErrorContains(t, err, "invalid url")
}

func TestNewCheckerRequiresTargetCategory(t *testing.T) {
	base := TargetConfig{Name: "gate", Type: TargetTypeTCP, Address: "127.0.0.1:1"}
	_, err := NewChecker(base)
	assert.ErrorContains(t, err, "targetCategory must be game|third_party")

	base.Category = "external"
	_, err = NewChecker(base)
	assert.ErrorContains(t, err, "targetCategory must be game|third_party")

	for _, category := range []string{TargetCategoryGame, TargetCategoryThirdParty} {
		base.Category = category
		_, err = NewChecker(base)
		require.NoError(t, err, "category %q", category)
	}
}

func TestProbeCategorySnapshot(t *testing.T) {
	p, err := ProbeFromTarget(TargetConfig{
		Name:          "gate",
		Category:      TargetCategoryThirdParty,
		Type:          TargetTypeTCP,
		Address:       "127.0.0.1:1",
		FailThreshold: 1,
	}, "demo", "prod", nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	result := p.ProbeOnce(ctx)
	assert.Equal(t, TargetCategoryThirdParty, result.Category)
	window, open := p.CurrentWindow()
	require.True(t, open)
	assert.Equal(t, TargetCategoryThirdParty, window.Category)

}

func TestNewCheckerTCP(t *testing.T) {
	chk, err := NewChecker(TargetConfig{Name: "game", Category: TargetCategoryGame, Type: TargetTypeTCP, Address: "127.0.0.1:19091"})
	require.NoError(t, err)
	_, ok := chk.(*TCPChecker)
	require.True(t, ok)

	_, err = NewChecker(TargetConfig{Name: "game", Category: TargetCategoryGame, Type: TargetTypeTCP})
	assert.ErrorContains(t, err, "requires address")
}

func TestNewCheckerDB(t *testing.T) {
	chk, err := NewChecker(TargetConfig{Name: "db", Category: TargetCategoryGame, Type: TargetTypeDB, DSN: "dsn", Driver: "mysql"})
	require.NoError(t, err)
	dbchk, ok := chk.(*DBChecker)
	require.True(t, ok)
	assert.Equal(t, "mysql", dbchk.Driver)

	_, err = NewChecker(TargetConfig{Name: "db", Category: TargetCategoryGame, Type: TargetTypeDB, DSN: "dsn"})
	assert.ErrorContains(t, err, "requires dsn and driver")
	_, err = NewChecker(TargetConfig{Name: "db", Category: TargetCategoryGame, Type: TargetTypeDB, Driver: "mysql"})
	assert.ErrorContains(t, err, "requires dsn and driver")
}

func TestNewCheckerUnknownType(t *testing.T) {
	_, err := NewChecker(TargetConfig{Name: "x", Category: TargetCategoryThirdParty, Type: "grpc"})
	assert.ErrorContains(t, err, "unknown type")
	// 类型大小写不敏感。
	_, err = NewChecker(TargetConfig{Name: "x", Category: TargetCategoryThirdParty, Type: "HTTP", URL: "http://127.0.0.1:1"})
	assert.NoError(t, err)
}

func TestProbeFromTargetDefaults(t *testing.T) {
	var windows []WindowEvent
	var results []Result
	p, err := ProbeFromTarget(TargetConfig{
		Name: "gate", Category: TargetCategoryGame, Type: TargetTypeTCP, Address: "127.0.0.1:1",
	}, "demo", "prod", func(ev WindowEvent) { windows = append(windows, ev) },
		func(r Result) { results = append(results, r) })
	require.NoError(t, err)

	// 语义默认 Readiness；scope 透传；回调已挂接。
	assert.Equal(t, SemanticsReadiness, p.sem)
	assert.Equal(t, "demo", p.scopeGame)
	assert.Equal(t, "prod", p.scopeEnv)
	assert.Equal(t, TargetCategoryGame, p.category)
	assert.NotNil(t, p.listener)
	assert.NotNil(t, p.resultFn)

	// fail=3/recover=1 默认阈值在 Probe 内。
	assert.Equal(t, 3, p.failThr)
	assert.Equal(t, 1, p.okThr)

	p.ProbeOnce(context.Background())
	assert.Empty(t, windows) // 单次失败不达阈值，无窗口事件
	require.Len(t, results, 1)
	assert.NotEmpty(t, results[0].Err, "connect should fail")
	assert.Equal(t, TargetCategoryGame, results[0].Category)
	// 两态机初始 available：单次失败不达阈值不变态。
	assert.True(t, results[0].Available)
	assert.Equal(t, 1, results[0].ConsecutiveFailures)
}

func TestProbeFromTargetExplicitOverrides(t *testing.T) {
	p, err := ProbeFromTarget(TargetConfig{
		Name: "gate", Category: TargetCategoryThirdParty, Type: TargetTypeTCP, Address: "127.0.0.1:1",
		Semantics:        SemanticsLiveness,
		FailThreshold:    2,
		RecoverThreshold: 2,
	}, "", "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, SemanticsLiveness, p.sem)
	assert.Equal(t, 2, p.failThr)
	assert.Equal(t, 2, p.okThr)
	assert.Nil(t, p.listener) // 回调可选
	assert.Nil(t, p.resultFn)
}

func TestProbeFromTargetValidation(t *testing.T) {
	// 无名目标拒绝。
	_, err := ProbeFromTarget(TargetConfig{Category: TargetCategoryGame, Type: TargetTypeTCP, Address: "127.0.0.1:1"}, "", "", nil, nil)
	assert.ErrorContains(t, err, "name is required")
	// 未知类型在工厂报错。
	_, err = ProbeFromTarget(TargetConfig{Name: "x", Category: TargetCategoryGame, Type: "ws"}, "", "", nil, nil)
	assert.ErrorContains(t, err, "unknown type")
	// 缺参目标在工厂报错。
	_, err = ProbeFromTarget(TargetConfig{Name: "x", Category: TargetCategoryGame, Type: TargetTypeHTTP}, "", "", nil, nil)
	assert.ErrorContains(t, err, "requires url")
}
