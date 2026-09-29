package cicd_test

// 注册表 fail-fast 契约（覆盖率巡检）：Register 对空 kind / nil 工厂必须
// panic——provider 类型注册是启动期一次性动作，误注册若静默放过，会在第一次
// New() 时才炸，故按编程错误 fail-fast。

import (
	"testing"

	"github.com/cuihairu/croupier/internal/cicd"
	"github.com/stretchr/testify/assert"
)

func TestRegister_EmptyKindOrNilFactoryPanics(t *testing.T) {
	assert.PanicsWithValue(t, "cicd: Register requires kind and factory", func() {
		cicd.Register("", func(cicd.Config) (cicd.Provider, error) { return nil, nil })
	})
	assert.PanicsWithValue(t, "cicd: Register requires kind and factory", func() {
		cicd.Register("probe-empty-kind", nil)
	})
	assert.PanicsWithValue(t, "cicd: Register requires kind and factory", func() {
		cicd.Register("probe-nil-factory", nil)
	})
	// 失败的注册尝试不得污染注册表
	assert.NotContains(t, cicd.Kinds(), "probe-empty-kind")
	assert.NotContains(t, cicd.Kinds(), "probe-nil-factory")
}
