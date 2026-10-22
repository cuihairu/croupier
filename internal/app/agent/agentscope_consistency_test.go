package agent

import (
	"testing"
	"time"

	captureconfig "github.com/cuihairu/croupier/agents/capture/config"
	devopsconfig "github.com/cuihairu/croupier/agents/devops/config"
	syntheticsconfig "github.com/cuihairu/croupier/agents/synthetics/config"
)

// TestFourAgentKindsRejectBlankGameScope（todo.md:29）：sidecar/devops/
// capture/synthetics 四类 agent 的显式归属口径一致——gameId/env 去空白后
// 必须非空，空/空白一律确定拒绝，不隐式回落 dev/prod/default。
// synthetics 的同断言另在 agents/synthetics/config 的无 tag 测试中覆盖，
// 保证 no_go_plugin 裁剪构建下同效。
func TestFourAgentKindsRejectBlankGameScope(t *testing.T) {
	blanks := []string{"", "   ", "\t\n"}

	// gameID(env string, blank string) 返回把 scope 字段替换成 blank 后的
	// 校验结果；valid 各返回该类最小合法配置（capture/sidecar 业务块缺省关）。
	kinds := []struct {
		name   string
		gameID func(blank string) error
		env    func(blank string) error
	}{
		{
			name: "sidecar",
			gameID: func(blank string) error {
				return ValidateAgentScope(blank, "prod")
			},
			env: func(blank string) error {
				return ValidateAgentScope("demo", blank)
			},
		},
		{
			name: "devops",
			gameID: func(blank string) error {
				c := devopsconfig.Config{Agent: devopsconfig.Agent{ID: "a", ServerAddr: "s", GameID: blank, Env: "prod"}}
				return c.Validate()
			},
			env: func(blank string) error {
				c := devopsconfig.Config{Agent: devopsconfig.Agent{ID: "a", ServerAddr: "s", GameID: "demo", Env: blank}}
				return c.Validate()
			},
		},
		{
			name: "capture",
			gameID: func(blank string) error {
				c := captureconfig.Config{Agent: captureconfig.Agent{ID: "a", ServerAddr: "s", GameID: blank, Env: "prod"}}
				return c.Validate()
			},
			env: func(blank string) error {
				c := captureconfig.Config{Agent: captureconfig.Agent{ID: "a", ServerAddr: "s", GameID: "demo", Env: blank}}
				return c.Validate()
			},
		},
		{
			name: "synthetics",
			gameID: func(blank string) error {
				c := syntheticsConfig(blank, "prod")
				return c.Validate()
			},
			env: func(blank string) error {
				c := syntheticsConfig("demo", blank)
				return c.Validate()
			},
		},
	}

	for _, k := range kinds {
		for _, blank := range blanks {
			if err := k.gameID(blank); err == nil {
				t.Errorf("%s: blank gameId=%q accepted", k.name, blank)
			}
			if err := k.env(blank); err == nil {
				t.Errorf("%s: blank env=%q accepted", k.name, blank)
			}
		}
		// 合法 scope 必须通过（防止“全都拒绝、合法也拒绝”的假收口）。
		if err := k.gameID("demo"); err != nil {
			t.Errorf("%s: valid gameId rejected: %v", k.name, err)
		}
		if err := k.env("prod"); err != nil {
			t.Errorf("%s: valid env rejected: %v", k.name, err)
		}
	}
}

// syntheticsConfig 构造最小合法 synthetics 配置（业务块合法，仅 scope 可注入）。
func syntheticsConfig(gameID, env string) syntheticsconfig.Config {
	return syntheticsconfig.Config{
		Agent: syntheticsconfig.Agent{ID: "a", ServerAddr: "s", GameID: gameID, Env: env},
		Synthetics: syntheticsconfig.Synthetics{
			Interval: time.Second,
			Targets:  []syntheticsconfig.Target{{Name: "t", URL: "http://127.0.0.1", Method: "GET", Timeout: time.Second, ExpectedStatus: 200}},
		},
	}
}
