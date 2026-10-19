package config

import "testing"

// TestValidateRejectsBlankGameScopeInEveryBuild：scope 校验不依赖构建组合
// （no_go_plugin 等裁剪构建下同样生效）——agent.id/gameId/env 去空白后必须
// 非空，空/空白一律确定拒绝，不隐式回落（todo.md:29 四类显式归属口径）。
// 本文件无 build tag：默认构建与裁剪构建都会运行。
func TestValidateRejectsBlankGameScopeInEveryBuild(t *testing.T) {
	blanks := []string{"", "   ", "\t\n"}
	for _, field := range []string{"id", "gameId", "env"} {
		for _, blank := range blanks {
			c := valid()
			switch field {
			case "id":
				c.Agent.ID = blank
			case "gameId":
				c.Agent.GameID = blank
			case "env":
				c.Agent.Env = blank
			}
			if err := c.Validate(); err == nil {
				t.Fatalf("blank %s=%q accepted", field, blank)
			}
		}
	}
}
