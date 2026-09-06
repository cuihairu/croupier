package routes

import (
	"context"
	"testing"
)

// extractObjectName 的 `return "other"` 分支（get_routes_logic.go:62）不可达：
// strings.Split 对任意输入（含空串）都返回至少 1 个元素，len(parts) > 0 恒真。
// 此测试固化该防御性死分支的行为边界。
func TestExtractObjectNameSplitSemantics(t *testing.T) {
	logic := &GetRoutesLogic{ctx: context.Background()}
	cases := map[string]string{
		"player.getList": "player",
		"single":         "single",
		"":               "",
		"a.b.c.d":        "a",
		".leading":       "",
		"trailing.":      "trailing",
		"player..double": "player",
	}
	for input, want := range cases {
		if got := logic.extractObjectName(input); got != want {
			t.Errorf("extractObjectName(%q) = %q, want %q", input, got, want)
		}
	}
}
