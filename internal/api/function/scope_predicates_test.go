package function

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cuihairu/croupier/internal/svc"
)

// 会话 scope 谓词：scope.GameID 为空 = 全匹配（兼容未注入）；非空 = game+env 双匹配。

func TestSessionMatchesScope(t *testing.T) {
	// 空 scope → 恒 true（不注入 scope 的调用方保持旧行为）。
	assert.True(t, sessionMatchesScope(svc.GameScope{}, "any", "any"))
	assert.True(t, sessionMatchesScope(svc.GameScope{Env: "prod"}, "g1", "dev"),
		"仅 Env 无 GameID 时按空 GameID 分支放行")

	// 非空 scope：game+env 必须都匹配（大小写/空白不敏感）。
	scope := svc.GameScope{GameID: "g1", Env: "prod"}
	assert.True(t, sessionMatchesScope(scope, "g1", "prod"))
	assert.True(t, sessionMatchesScope(scope, " G1 ", " PROD "))

	// 跨 game / 跨 env / 只 game 对 / 只 env 对 → 拒绝。
	assert.False(t, sessionMatchesScope(scope, "g2", "prod"))
	assert.False(t, sessionMatchesScope(scope, "g1", "dev"))
	assert.False(t, sessionMatchesScope(scope, "", "prod"))
	assert.False(t, sessionMatchesScope(scope, "g1", ""))
	assert.False(t, sessionMatchesScope(scope, "", ""))
}
