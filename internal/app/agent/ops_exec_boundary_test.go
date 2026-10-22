package agent

// 远程 Shell 边界物化（agent-supervisor-design §10.7 三件套前置之一：白名单）。
// allowExec=true 而 execAllowedCommands 为空曾在运行期等价「放行一切」——与
// 「白名单前置」边界相反。本组用例钉死两条防线：配置校验 fail-fast + 运行期
// fail-closed（绕过 Validate 直构实例也拒绝）。

import (
	"context"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAllowExecRequiresWhitelist(t *testing.T) {
	// allowExec 开 + 白名单空 → 配置校验失败（白名单是硬前置，不许隐式放行一切）。
	allowAll := DefaultOpsConfig()
	allowAll.Enabled = true
	allowAll.AllowExec = true
	err := allowAll.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execAllowedCommands")

	// 白名单非空 → 通过。
	whitelisted := DefaultOpsConfig()
	whitelisted.Enabled = true
	whitelisted.AllowExec = true
	whitelisted.ExecAllowedCommands = []string{"echo"}
	assert.NoError(t, whitelisted.Validate())

	// allowExec 关 + 白名单空 → 正常（exec 面整体关闭）。
	closed := DefaultOpsConfig()
	closed.Enabled = true
	assert.NoError(t, closed.Validate())
}

func TestExecuteCommandFailsClosedWithoutWhitelist(t *testing.T) {
	// 运行期防线：绕过 Validate 直构（程序内装配路径）也必须拒绝——
	// 空白名单即拒绝一切，不回退「放行一切」旧语义。
	s := NewOpsServer(&OpsConfig{
		Enabled:     true,
		AllowExec:   true,
		ExecTimeout: 5 * time.Second,
		// ExecAllowedCommands 故意留空
	}, "agent-exec-boundary", "test", nil)

	_, err := s.ExecuteCommand(context.Background(), &opsv1.ExecuteCommandRequest{Command: "echo"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "whitelist")
}

func TestExecuteCommandWhitelistStillAllowsListed(t *testing.T) {
	// 白名单内的命令照常执行（fail-closed 不误伤显式声明）。
	s := NewOpsServer(&OpsConfig{
		Enabled:             true,
		AllowExec:           true,
		ExecTimeout:         5 * time.Second,
		ExecAllowedCommands: []string{"echo"},
	}, "agent-exec-boundary", "test", nil)

	resp, err := s.ExecuteCommand(context.Background(), &opsv1.ExecuteCommandRequest{Command: "echo"})
	require.NoError(t, err)
	assert.Zero(t, resp.ExitCode)
}
