package policy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

// 本文件覆盖 manager.go 中两处 marshal 失败分支。
// json.Marshal 对 []string 不存在可构造的失败输入（非法 UTF-8 会被替换为
// U+FFFD 而非报错），因此通过 marshalRoles 缝隙注入故障触达错误分支。

// TestJSONMarshalStringSliceNeverFails 逐一验证可能导致怀疑的输入形态，
// 证明 json.Marshal 对 []string 恒返回 nil error。
func TestJSONMarshalStringSliceNeverFails(t *testing.T) {
	cases := [][]string{
		nil,
		{},
		{"admin"},
		{"管理员", "オペレーター", "👍"},
		{"invalid utf-8: \xff\xfe"},
		{"a", "a", "a"},
	}
	for _, roles := range cases {
		_, err := json.Marshal(roles)
		assert.NoError(t, err, "json.Marshal must never fail on []string: %#v", roles)
	}
}

// TestSetOverrideMarshalRolesInvariant 端到端复核：含极端角色列表的覆盖策略
// 与全风险等级的默认策略写入均恒成功（marshal 错误分支不可自然触达）。
func TestSetOverrideMarshalRolesInvariant(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// setupTestDB 使用 file::memory:?cache=shared，记录会跨测试泄漏；
	// 结束时清理本测试写入的行，避免污染后续 TestListOverrides 的计数。
	t.Cleanup(func() {
		db.Where("function_id LIKE ?", "fn.marshal.%").Delete(&model.FunctionPolicy{})
	})

	require.NoError(t, m.SetOverride(ctx, "fn.marshal.nil", &Policy{AllowedRoles: nil}))
	require.NoError(t, m.SetOverride(ctx, "fn.marshal.utf8", &Policy{AllowedRoles: []string{"角色\xff"}}))

	for _, risk := range []RiskLevel{RiskLow, RiskMedium, RiskHigh, RiskDanger, RiskUnknown} {
		require.NoError(t, m.EnsureDefaultPolicy(ctx, "fn.marshal."+string(risk), risk))
	}
}

// TestSetOverrideMarshalError 注入 marshal 失败，覆盖 SetOverride 与
// EnsureDefaultPolicy 的错误返回分支，并断言失败时无脏写入库。
func TestSetOverrideMarshalError(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	orig := marshalRoles
	marshalRoles = func([]string) ([]byte, error) { return nil, errors.New("injected marshal failure") }
	t.Cleanup(func() { marshalRoles = orig })

	assert.Error(t, m.SetOverride(ctx, "fn.marshal.fail", &Policy{AllowedRoles: []string{"admin"}}))
	assert.Error(t, m.EnsureDefaultPolicy(ctx, "fn.marshal.fail.default", RiskHigh))

	var count int64
	require.NoError(t, db.Model(&model.FunctionPolicy{}).Where("function_id LIKE ?", "fn.marshal.fail%").Count(&count).Error)
	assert.Zero(t, count, "marshal 失败时不应有任何记录落库")
}
