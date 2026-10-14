package agent

// 受管程序二进制更新用例（agent-supervisor-design §10.4 更新状态+指纹）：
// 内容哈希 fail-closed（不匹配拒绝、不碰运行态）、先停后换、稳定性窗口内
// 退出即回滚（rolledBackTo=旧指纹留痕）、指纹随快照上行。签名校验/兼容性
// 检查/游戏维护窗口是诚实边界，不在本批（见 ops_update.go 头注）。

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func updateTestServer(t *testing.T, scriptPath, oldScript string, window time.Duration) *OpsServer {
	t.Helper()
	require.NoError(t, os.WriteFile(scriptPath, []byte(oldScript), 0o755))
	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowUpdate:   true,
		AllowRestart:  true,
		SupervisorLog: desiredTestDir(t),
		ManagedProcesses: map[string]ManagedProcessConfig{
			"demo-app": {Command: scriptPath, AutoRestart: true},
		},
	}, "agent-a", "test", nil)
	s.updateStabilityWindow = window
	require.NoError(t, s.Start())
	t.Cleanup(s.Stop)
	return s
}

func TestUpdateProcessRequiresGate(t *testing.T) {
	s := NewOpsServer(&OpsConfig{
		Enabled: true,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"demo-app": {Command: "sleep", Args: []string{"30"}},
		},
	}, "agent-a", "test", nil)

	_, err := s.UpdateProcess(context.Background(), &opsv1.UpdateProcessRequest{
		ProcessName:    "demo-app",
		StagedPath:     "/tmp/x",
		ExpectedSha256: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestUpdateProcessHashMismatchRefuses(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "app.sh")
	s := updateTestServer(t, script, "#!/bin/sh\n# v1\nsleep 30\n", time.Second)

	require.Eventually(t, func() bool {
		return s.SampleSupervisedProcesses()[0].GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING
	}, 3*time.Second, 20*time.Millisecond)

	staged := filepath.Join(dir, "staged")
	require.NoError(t, os.WriteFile(staged, []byte("#!/bin/sh\n# v2\nsleep 30\n"), 0o755))

	_, err := s.UpdateProcess(context.Background(), &opsv1.UpdateProcessRequest{
		ProcessName:    "demo-app",
		StagedPath:     staged,
		ExpectedSha256: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")

	// fail-closed：目标文件未动，运行中进程未被打断。
	raw, readErr := os.ReadFile(script)
	require.NoError(t, readErr)
	assert.Contains(t, string(raw), "# v1")
	assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_RUNNING, s.SampleSupervisedProcesses()[0].GetState())
}

func TestUpdateProcessSwapsBinaryAndRestarts(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "app.sh")
	oldV1 := "#!/bin/sh\n# v1\nsleep 30\n"
	s := updateTestServer(t, script, oldV1, 300*time.Millisecond)

	require.Eventually(t, func() bool {
		return s.SampleSupervisedProcesses()[0].GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING
	}, 3*time.Second, 20*time.Millisecond)
	oldSnap := s.SampleSupervisedProcesses()[0]
	oldFP := oldSnap.GetFingerprint()
	require.NotEmpty(t, oldFP, "snapshot carries the running binary fingerprint")

	staged := filepath.Join(dir, "staged")
	newScript := "#!/bin/sh\n# v2\nsleep 30\n"
	require.NoError(t, os.WriteFile(staged, []byte(newScript), 0o755))
	newFP := sha256HexFile(staged)
	require.NotEqual(t, oldFP, newFP)

	resp, err := s.UpdateProcess(context.Background(), &opsv1.UpdateProcessRequest{
		ProcessName:    "demo-app",
		StagedPath:     staged,
		ExpectedSha256: newFP,
	})
	require.NoError(t, err)
	require.True(t, resp.GetSuccess())
	assert.False(t, resp.GetRolledBack())
	assert.Equal(t, oldFP, resp.GetOldFingerprint())
	assert.Equal(t, newFP, resp.GetNewFingerprint())

	// 目标文件已换新内容；旧内容备份保留为 last-known-good（人工回滚）。
	raw, readErr := os.ReadFile(script)
	require.NoError(t, readErr)
	assert.Equal(t, newScript, string(raw))
	backupRaw, backupErr := os.ReadFile(script + updateBackupSuffix)
	require.NoError(t, backupErr, "backup must be retained as last-known-good")
	assert.Equal(t, oldV1, string(backupRaw))

	// 新实例运行中，指纹随快照更新。
	require.Eventually(t, func() bool {
		snap := s.SampleSupervisedProcesses()[0]
		return snap.GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING && snap.GetFingerprint() == newFP
	}, 3*time.Second, 20*time.Millisecond)

	running, ok := s.desired.Running("demo-app")
	assert.True(t, ok)
	assert.True(t, running, "update start is an explicit run intent")

	// update_applied 留痕（message 带旧→新指纹）。
	found := false
	for _, ev := range s.supLog.EventsSince(0) {
		if ev.GetEvent() == "update_applied" && ev.GetMessage() == "binary updated sha256 "+oldFP+" -> "+newFP {
			found = true
		}
	}
	assert.True(t, found, "update_applied event must record old -> new fingerprint")
}

func TestUpdateProcessRollsBackOnFailedStart(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "app.sh")
	oldScript := "#!/bin/sh\n# v1\nsleep 30\n"
	s := updateTestServer(t, script, oldScript, 250*time.Millisecond)

	require.Eventually(t, func() bool {
		return s.SampleSupervisedProcesses()[0].GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING
	}, 3*time.Second, 20*time.Millisecond)

	staged := filepath.Join(dir, "staged")
	brokenScript := "#!/bin/sh\nexit 3\n" // 稳定性窗口内必退出
	require.NoError(t, os.WriteFile(staged, []byte(brokenScript), 0o755))
	brokenFP := sha256HexFile(staged)

	resp, err := s.UpdateProcess(context.Background(), &opsv1.UpdateProcessRequest{
		ProcessName:    "demo-app",
		StagedPath:     staged,
		ExpectedSha256: brokenFP,
	})
	require.NoError(t, err)
	require.False(t, resp.GetSuccess())
	assert.True(t, resp.GetRolledBack(), "failed start must roll back")
	assert.Equal(t, sha256HexFile(script), resp.GetOldFingerprint())
	assert.Equal(t, brokenFP, resp.GetNewFingerprint())

	// 旧二进制内容恢复，进程重新运行旧指纹。
	raw, readErr := os.ReadFile(script)
	require.NoError(t, readErr)
	assert.Equal(t, oldScript, string(raw))

	require.Eventually(t, func() bool {
		snap := s.SampleSupervisedProcesses()[0]
		return snap.GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING && snap.GetFingerprint() == resp.GetOldFingerprint()
	}, 3*time.Second, 20*time.Millisecond)

	// update_rolled_back 留痕（rolledBackTo=旧指纹）。
	found := false
	for _, ev := range s.supLog.EventsSince(0) {
		if ev.GetEvent() == "update_rolled_back" && ev.GetMessage() == "binary update rolled back to sha256 "+resp.GetOldFingerprint()+" (update reason: new binary exited within stability window)" {
			found = true
		}
	}
	assert.True(t, found, "update_rolled_back event must record the rollback fingerprint")
}

func TestUpdateProcessStagedFileUnreadable(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "app.sh")
	s := updateTestServer(t, script, "#!/bin/sh\n# v1\nsleep 30\n", time.Second)

	_, err := s.UpdateProcess(context.Background(), &opsv1.UpdateProcessRequest{
		ProcessName:    "demo-app",
		StagedPath:     filepath.Join(dir, "missing"),
		ExpectedSha256: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unreadable")
}

func TestSnapshotFingerprintOfFreshProcess(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "app.sh")
	s := updateTestServer(t, script, "#!/bin/sh\n# v1\nsleep 30\n", time.Second)

	require.Eventually(t, func() bool {
		return s.SampleSupervisedProcesses()[0].GetState() == opsv1.ProcessState_PROCESS_STATE_RUNNING
	}, 3*time.Second, 20*time.Millisecond)

	// §10.4 指纹硬要求：运行实例指纹=二进制内容 sha256，随快照上行。
	snap := s.SampleSupervisedProcesses()[0]
	assert.Equal(t, sha256HexFile(script), snap.GetFingerprint())
}
