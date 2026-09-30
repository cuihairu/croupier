// 覆盖率巡检第二十二轮（wt-api）：cmd/server 小文件 CLI 翼收口——
// health/validate/version/completion 四命令 RunE、db fanout 四路径、
// meshForwarder 的 !OK 返回翼与 callerFromContext nil-ctx 翼、
// remote_agent_source 的 ActiveAgentIDs 错误翼。
// 本包无 t.Parallel（已核实），全局 cobra 变量（cfgFile/bootstrapDataDir/
// dbFanoutDryRun/GitCommit/BuildTime）操纵安全，t.Cleanup 恢复。
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCfgR22(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func withGlobalsR22(t *testing.T, cfg string, bootstrapDir string, dryRun bool) {
	t.Helper()
	oldCfg, oldDir, oldDry := cfgFile, bootstrapDataDir, dbFanoutDryRun
	cfgFile, bootstrapDataDir, dbFanoutDryRun = cfg, bootstrapDir, dryRun
	t.Cleanup(func() {
		cfgFile, bootstrapDataDir, dbFanoutDryRun = oldCfg, oldDir, oldDry
	})
}

func TestHealthCmd_RunE_Wings(t *testing.T) {
	// NewServiceContext 对空 DataSource 回落 data/croupier.db（相对 cwd
	// 自动建库），t.Chdir 隔离避免污染包目录。
	t.Chdir(t.TempDir())

	// DB 已配置 + JWT 已配置 + bootstrapDataDir 覆盖翼
	dbPath := filepath.Join(t.TempDir(), "health.db")
	cfg := writeCfgR22(t, "server:\n  host: 127.0.0.1\n  port: 18780\n"+
		"database:\n  driver: sqlite\n  dataSource: "+dbPath+"\n"+
		"auth:\n  jwtSecret: r22-health-secret-0123456789\n")
	withGlobalsR22(t, cfg, filepath.Join(t.TempDir(), "boot"), false)

	out := captureServerStdout(t, func() {
		require.NoError(t, healthCmd.RunE(healthCmd, nil))
	})
	assert.Contains(t, out, "数据库配置: sqlite")
	assert.Contains(t, out, "JWT密钥已配置")
	assert.Contains(t, out, "服务上下文初始化成功")

	// DB 未配置 + JWT 未配置双警告翼：空 DataSource 经 driver=auto 回落
	// sqlite 默认 data/croupier.db，健康命令仍按配置面打印警告。
	cfgNoDB := writeCfgR22(t, "server:\n  host: 127.0.0.1\n  port: 18780\n")
	withGlobalsR22(t, cfgNoDB, "", false)
	out = captureServerStdout(t, func() {
		require.NoError(t, healthCmd.RunE(healthCmd, nil))
	})
	assert.Contains(t, out, "数据库未配置")
	assert.Contains(t, out, "JWT密钥未配置")

	// 配置文件缺失 → loadConfigFile 错误上抛
	withGlobalsR22(t, filepath.Join(t.TempDir(), "missing.yaml"), "", false)
	require.Error(t, healthCmd.RunE(healthCmd, nil))
}

func TestValidateCmd_RunE_AndMask(t *testing.T) {
	// multiGame 启用翼 + 长密钥掩码尾 4 位
	cfg := writeCfgR22(t, "server:\n  host: 127.0.0.1\n  port: 18780\n  mode: dev\n"+
		"database:\n  driver: sqlite\n  dataSource: "+filepath.Join(t.TempDir(), "v.db")+"\n  multiGame: true\n"+
		"auth:\n  jwtSecret: r22-validate-secret-abcdef\n")
	withGlobalsR22(t, cfg, "", false)

	out := captureServerStdout(t, func() {
		require.NoError(t, validateCmd.RunE(validateCmd, nil))
	})
	assert.Contains(t, out, "多游戏分库: 启用")
	assert.Contains(t, out, "***cdef")

	// maskIfSet 三翼
	assert.Equal(t, "未设置", maskIfSet(""))
	assert.Equal(t, "已设置", maskIfSet("short"))
	assert.Equal(t, "已设置 (***wxyz)", maskIfSet("0123456789wxyz"))

	// 配置文件缺失 → loadConfigFile 错误上抛
	withGlobalsR22(t, filepath.Join(t.TempDir(), "missing.yaml"), "", false)
	require.Error(t, validateCmd.RunE(validateCmd, nil))
}

func TestVersionCmd_PrintWings(t *testing.T) {
	oldCommit, oldBuild := GitCommit, BuildTime
	GitCommit, BuildTime = "r22abc", "2026-09-29T00:00:00Z"
	t.Cleanup(func() { GitCommit, BuildTime = oldCommit, oldBuild })

	out := captureServerStdout(t, func() { versionCmd.Run(versionCmd, nil) })
	assert.Contains(t, out, "Croupier Server")
	assert.Contains(t, out, "Git commit: r22abc")
	assert.Contains(t, out, "Build time: 2026-09-29T00:00:00Z")

	// ldflags 缺省形态：unknown/空 → 不打印 commit/build 行
	GitCommit, BuildTime = "unknown", ""
	out = captureServerStdout(t, func() { versionCmd.Run(versionCmd, nil) })
	assert.Contains(t, out, "Croupier Server")
	assert.NotContains(t, out, "Git commit")
	assert.NotContains(t, out, "Build time")
}

func TestCompletionCmd_FourShells(t *testing.T) {
	// 生成脚本走 os.Stdout，包一层捕获防测试日志被刷屏
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		shell := shell
		_ = captureServerStdout(t, func() {
			require.NoError(t, completionCmd.RunE(completionCmd, []string{shell}),
				"shell %s 生成失败", shell)
		})
	}
	// default 分支登记不可达：Args=OnlyValidArgs 在 RunE 前拦截
	// （四词之外任何参数到不了 switch 的 default 臂），证明性断言锁定前提
	require.Error(t, completionCmd.Args(completionCmd, []string{"tcsh"}),
		"非法 shell 应被参数校验拦截（default 臂不可达前提）")
}

// meshForwarder 的 !OK 返回翼：owner 解析为本实例 → mesh 防御性回
// OK=false("owner is self")，forwarder 须包装为 remote invoke failed。
// NotOwner/成功载荷两翼登记不可达：只经真实 peer 环路（transport 拨号）
// 触达，cluster 层 interconnect e2e 已覆盖同语义（coverage_v9 断言
// "owner is self" 原文）。
type selfOwnerResolverR22 struct{}

func (selfOwnerResolverR22) ResolveOwner(context.Context, string) (*cluster.PeerInfo, error) {
	self := cluster.PeerInfo{InstanceID: "self", Epoch: 1}
	return &self, nil
}

func TestMeshForwarder_RemoteInvokeFailedWing(t *testing.T) {
	mesh := cluster.NewMeshInterconnect(cluster.PeerInfo{InstanceID: "self"}, selfOwnerResolverR22{}, nil)
	f := newMeshForwarder(mesh)

	_, err := f.Forward(context.Background(), &dispatch.RemoteCall{
		AgentID: "a1", FunctionID: "demo.fn",
	})
	require.ErrorContains(t, err, "remote invoke failed: owner is self")
}

func TestCallerFromContext_NilCtxWing(t *testing.T) {
	caller := callerFromContext(nil, map[string]string{"gameId": "g", "env": "e"})
	assert.Equal(t, "g", caller.GameID)
	assert.Equal(t, "e", caller.Env)
	assert.Empty(t, caller.Username, "nil ctx 早退：不读身份键")
}

// remote_agent_source ActiveAgentIDs 的 ListAliveOwners 错误翼。
type failingListOwnersStoreR22 struct {
	cluster.OwnerStore
}

func (failingListOwnersStoreR22) ListAliveOwners(context.Context) ([]cluster.AgentOwnerRecord, error) {
	return nil, errors.New("r22: list owners boom")
}

func TestActiveAgentIDs_ListOwnersErrorWing(t *testing.T) {
	_, err := activeAgentIDDirectory{resolver: failingListOwnersStoreR22{}}.ActiveAgentIDs(context.Background())
	require.ErrorContains(t, err, "r22: list owners boom")
}

func runDbFanoutR22(t *testing.T) (string, error) {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var err error
	out := captureServerStdout(t, func() {
		err = dbFanoutCmd.RunE(cmd, nil)
	})
	return out, err
}

func TestDbFanoutCmd_RunE_Paths(t *testing.T) {
	// dry-run：合法 sqlite meta 库 → 报告表格打印（含 (meta) 行）、无错误
	cfg := writeCfgR22(t, "server:\n  host: 127.0.0.1\n  port: 18780\n"+
		"database:\n  driver: sqlite\n  dataSource: "+filepath.Join(t.TempDir(), "meta.db")+"\n")
	withGlobalsR22(t, cfg, "", true)

	out, err := runDbFanoutR22(t)
	require.NoError(t, err)
	assert.Contains(t, out, "(meta)")
	assert.Contains(t, out, "total=1")

	// 配置文件缺失 → loadConfigFile 错误翼
	withGlobalsR22(t, filepath.Join(t.TempDir(), "nope.yaml"), "", false)
	_, err = runDbFanoutR22(t)
	require.Error(t, err)

	// meta 库打不开（DSN 是目录）→ fanout: open meta database 错误上抛
	//（经报告打印之后的 return err 路径）
	cfgDir := writeCfgR22(t, "server:\n  host: 127.0.0.1\n  port: 18780\n"+
		"database:\n  driver: sqlite\n  dataSource: "+t.TempDir()+"\n")
	withGlobalsR22(t, cfgDir, "", false)
	out, err = runDbFanoutR22(t)
	require.ErrorContains(t, err, "fanout: open meta database")
	assert.Contains(t, out, "total=0", "打开失败时报告通道仍打印空表")

	// 迁移被拒 → 报告 status=error → ErrFanoutFailures（错误经报告
	// 通道而非 RunMigrationFanout 返回值）：DSN 带 _pragma=query_only(1)
	// 使每条连接只读，Open/ping 成功而首个 CREATE TABLE 被拒。
	dir := t.TempDir()
	refusePath := filepath.Join(dir, "refuse.db")
	require.NoError(t, os.WriteFile(refusePath, nil, 0o600))
	cfgRO := writeCfgR22(t, "server:\n  host: 127.0.0.1\n  port: 18780\n"+
		"database:\n  driver: sqlite\n  dataSource: file:"+refusePath+"?_pragma=query_only(1)\n")
	withGlobalsR22(t, cfgRO, "", false)

	out, err = runDbFanoutR22(t)
	require.ErrorIs(t, err, svc.ErrFanoutFailures)
	assert.Contains(t, out, "error=1")
}
