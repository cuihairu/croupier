// 覆盖率巡检第十八轮（wt-api）：config.go LocalEnabled 归一化方法
// 收口（964e40b 新增、消费方在 api/auth，本包内零覆盖）——
// nil → true（默认启用，停用须显式 false）、显式 true/false 透传。
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalProviderConfig_LocalEnabled(t *testing.T) {
	assert.True(t, LocalProviderConfig{}.LocalEnabled(), "nil 视为默认启用")

	enabled := true
	assert.True(t, LocalProviderConfig{Enabled: &enabled}.LocalEnabled())

	disabled := false
	assert.False(t, LocalProviderConfig{Enabled: &disabled}.LocalEnabled(),
		"停用必须显式 false（防 YAML 省略键误停本地登录）")
}
