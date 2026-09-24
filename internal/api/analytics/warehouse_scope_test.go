package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// warehouse scope 片段 + 参数必须与占位符一一对应；遗漏 = 跨游戏数据泄漏。

func TestWarehouseScopeSuffix(t *testing.T) {
	assert.Empty(t, warehouseScopeSuffix("", ""), "空 scope 不加过滤（调用方负责全集语义）")
	assert.Equal(t, " AND game_id = ?", warehouseScopeSuffix("demo", ""))
	assert.Equal(t, " AND env = ?", warehouseScopeSuffix("", "prod"))
	assert.Equal(t, " AND game_id = ? AND env = ?", warehouseScopeSuffix("demo", "prod"))
}

func TestWarehouseScopeArgs_MatchesPlaceholders(t *testing.T) {
	cases := []struct {
		gameID, env string
		want        []any
	}{
		{"", "", []any{}},
		{"demo", "", []any{"demo"}},
		{"", "prod", []any{"prod"}},
		{"demo", "prod", []any{"demo", "prod"}},
	}
	for _, tc := range cases {
		suffix := warehouseScopeSuffix(tc.gameID, tc.env)
		args := warehouseScopeArgs(tc.gameID, tc.env)
		placeholders := 0
		for _, r := range suffix {
			if r == '?' {
				placeholders++
			}
		}
		assert.Len(t, args, placeholders,
			"suffix=%q 的占位符数必须等于 args 数", suffix)
		assert.Equal(t, tc.want, args)
	}
}
