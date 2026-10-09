// S3 崩溃快照测试：snapshotProfile 闭集校验、spawn 注入（env/argv）、快照
// 目录 0700、core_pattern 只读提示闭集、产物登记（快照目录+node 报告）、
// detect_down 事件携带快照上下文、快照 profile 进监管采样。
package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSnapshotProfileClosure(t *testing.T) {
	for _, p := range []string{"", "none", "go", "node", "python", "jvm"} {
		cfg := &OpsConfig{ManagedProcesses: map[string]ManagedProcessConfig{"a": {SnapshotProfile: p}}}
		require.NoError(t, cfg.Validate(), "profile %q should pass", p)
	}
	cfg := &OpsConfig{ManagedProcesses: map[string]ManagedProcessConfig{"a": {SnapshotProfile: "golang"}}}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "managedProcesses[a]")
	assert.Contains(t, err.Error(), "snapshotProfile")
}

func TestSnapshotEnvPerProfile(t *testing.T) {
	dir := "/snap/game"
	assert.Equal(t, []string{"GOTRACEBACK=crash"}, snapshotEnv(ManagedProcessConfig{SnapshotProfile: "go"}, dir))
	assert.Equal(t, []string{"PYTHONFAULTHANDLER=1"}, snapshotEnv(ManagedProcessConfig{SnapshotProfile: "python"}, dir))
	jvm := snapshotEnv(ManagedProcessConfig{SnapshotProfile: "jvm"}, dir)
	require.Len(t, jvm, 1)
	assert.Contains(t, jvm[0], "JAVA_TOOL_OPTIONS=-XX:+HeapDumpOnOutOfMemoryError")
	assert.Contains(t, jvm[0], "-XX:HeapDumpPath=/snap/game")
	assert.Empty(t, snapshotEnv(ManagedProcessConfig{SnapshotProfile: "none"}, dir))
	assert.Empty(t, snapshotEnv(ManagedProcessConfig{}, dir))
}

func TestSnapshotArgsNodeOnly(t *testing.T) {
	assert.Nil(t, snapshotArgs(ManagedProcessConfig{SnapshotProfile: "go"}))
	args := snapshotArgs(ManagedProcessConfig{SnapshotProfile: "node"})
	assert.Equal(t, snapshotNodeArgs, args)
}

func TestApplySnapshotProfileMutatesCmd(t *testing.T) {
	cmd := exec.Command("node", "server.js", "--port", "1")
	applySnapshotProfile(cmd, ManagedProcessConfig{SnapshotProfile: "node"}, "/snap")
	// argv 前置：node --report-on-fatalerror --report-exclude-env server.js --port 1
	require.Len(t, cmd.Args, 6)
	assert.Equal(t, "node", cmd.Args[0])
	assert.Equal(t, snapshotNodeArgs, cmd.Args[1:3])
	assert.Equal(t, []string{"server.js", "--port", "1"}, cmd.Args[3:])
	// 已带 flag 时不重复注入。
	cmd2 := exec.Command("node", "--report-on-fatalerror", "server.js")
	applySnapshotProfile(cmd2, ManagedProcessConfig{SnapshotProfile: "node"}, "/snap")
	require.Len(t, cmd2.Args, 3)
	// jvm 档 env 追加（档位优先于既有 env）。
	cmd3 := exec.Command("java", "-jar", "x.jar")
	cmd3.Env = []string{"PATH=/bin"}
	applySnapshotProfile(cmd3, ManagedProcessConfig{SnapshotProfile: "jvm"}, "/snap/jvm")
	require.Len(t, cmd3.Env, 2)
	assert.Contains(t, cmd3.Env[1], "JAVA_TOOL_OPTIONS=-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/snap/jvm")
}

func TestEnsureProcessSnapshotDir(t *testing.T) {
	base := t.TempDir()
	dir := ensureProcessSnapshotDir(base, "game")
	require.NotEmpty(t, dir)
	st, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, st.IsDir())
	assert.Equal(t, os.FileMode(0o700), st.Mode().Perm())
	// 幂等：再次创建返回同一路径。
	assert.Equal(t, dir, ensureProcessSnapshotDir(base, "game"))
	// base 是文件 → 失败返回空串（不阻断启动）。
	fileBase := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(fileBase, []byte("x"), 0o600))
	assert.Empty(t, ensureProcessSnapshotDir(fileBase, "game"))
}

func TestCorePatternClassification(t *testing.T) {
	assert.True(t, corePatternPipeMode("|/usr/lib/systemd/systemd-coredump %P %u %g %s %t %c %e"))
	assert.False(t, corePatternPipeMode("core"))
	assert.False(t, corePatternPipeMode("/tmp/core.%p"))
	assert.False(t, corePatternPipeMode(""))
}

func TestSnapshotHintNeededAndEvent(t *testing.T) {
	assert.False(t, snapshotHintNeeded("|/usr/lib/systemd/systemd-coredump %P", true))
	assert.False(t, snapshotHintNeeded("core", false))
	assert.False(t, snapshotHintNeeded("", true))
	assert.True(t, snapshotHintNeeded("core", true))

	ev := snapshotHintEvent("core")
	assert.Equal(t, snapshotEventHint, ev.Event)
	assert.Empty(t, ev.Process) // 宿主机级事件
	assert.Contains(t, ev.Message, `core_pattern="core"`)
	assert.Contains(t, ev.Message, "sysctl")
	assert.Contains(t, ev.Message, corePatternSuggestedCommand())
}

func TestCollectSnapshotArtifacts(t *testing.T) {
	snap := t.TempDir()
	work := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(snap, "heap.hprof"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(snap, "notes.txt"), []byte("x"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(snap, "subdir"), 0o700)) // 目录不算产物
	files := collectSnapshotArtifacts(snap, work, "jvm")
	assert.Equal(t, []string{"heap.hprof", "notes.txt"}, files)

	// node 报告在 WorkingDir 中按 mtime 倒序、有上限。
	require.NoError(t, os.WriteFile(filepath.Join(work, "report.1.json"), []byte("{}"), 0o600))
	older := time.Now().Add(-time.Hour)
	require.NoError(t, os.WriteFile(filepath.Join(work, "report.2.json"), []byte("{}"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(work, "report.2.json"), older, older))
	for i := 0; i < 7; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(work, "report.extra."+string(rune('a'+i))+".json"), []byte("{}"), 0o600))
	}
	nodeFiles := collectSnapshotArtifacts(snap, work, "node")
	foundReport := 0
	for _, f := range nodeFiles {
		if filepath.Ext(f) == ".json" {
			foundReport++
		}
	}
	assert.LessOrEqual(t, foundReport, snapshotReportMax)
	assert.Contains(t, nodeFiles, "report.1.json")

	// 目录为空/未建 → 空列表；路径空 → nil。
	assert.Empty(t, collectSnapshotArtifacts(t.TempDir(), work, "go"))
	assert.Nil(t, collectSnapshotArtifacts("", work, "node"))
}

func TestSnapshotProfileOnSample(t *testing.T) {
	s := newS2TestServer(t, &OpsConfig{
		ManagedProcesses: map[string]ManagedProcessConfig{
			"cold": {Command: "sh", SnapshotProfile: "go"}, // 配置面，从未启动
		},
	})
	snaps := s.SampleSupervisedProcesses()
	require.Len(t, snaps, 1)
	assert.Equal(t, "go", snaps[0].SnapshotProfile)
}

func TestDetectDownCarriesSnapshotArtifacts(t *testing.T) {
	base := t.TempDir()
	// 预置产物：startProcess 的 MkdirAll 幂等，预置文件保留。
	require.NoError(t, os.MkdirAll(filepath.Join(base, "game"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(base, "game", "dump.hprof"), []byte("x"), 0o600))

	s := newS2TestServer(t, &OpsConfig{
		SnapshotDir: base,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"game": {
				Command:               "sh",
				Args:                  []string{"-c", "exit 1"},
				SnapshotProfile:       "jvm",
				AutoRestart:           true,
				RestartBackoffInitial: time.Second,
				RestartBackoffMax:     time.Second,
				RestartBreakerLimit:   0,
			},
		},
	})
	require.NoError(t, s.Start())
	defer stopManaged(t, s, "game")

	ev := waitSupervisorEvent(t, s, supervisorEventDetectDown, 5*time.Second)
	require.NotNil(t, ev)
	assert.Equal(t, filepath.Join(base, "game"), ev.SnapshotDir)
	assert.Contains(t, ev.SnapshotFiles, "dump.hprof")
}

func TestEmitSnapshotHintOnce(t *testing.T) {
	orig := readCorePattern
	readCorePattern = func() string { return "core" }
	t.Cleanup(func() { readCorePattern = orig })

	s := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: SupervisorLogConfig{Dir: t.TempDir()},
		ManagedProcesses: map[string]ManagedProcessConfig{
			"app": {Command: "sh", SnapshotProfile: "go"},
		},
	}, "a", "v", nil)
	s.emitSnapshotHintOnce()
	s.emitSnapshotHintOnce() // once：重复调用不刷屏
	evs := s.SupervisorEventsSince(0)
	require.Len(t, evs, 1)
	assert.Equal(t, snapshotEventHint, evs[0].Event)

	// 无轻档进程 → 不发提示。
	s2 := NewOpsServer(&OpsConfig{
		Enabled:       true,
		AllowRestart:  true,
		SupervisorLog: SupervisorLogConfig{Dir: t.TempDir()},
		ManagedProcesses: map[string]ManagedProcessConfig{
			"plain": {Command: "sh"},
		},
	}, "a", "v", nil)
	s2.emitSnapshotHintOnce()
	assert.Empty(t, s2.SupervisorEventsSince(0))
}

func TestSnapshotDirOrDefault(t *testing.T) {
	assert.Equal(t, "logs/snapshots", (&OpsConfig{}).SnapshotDirOrDefault())
	assert.Equal(t, "/var/snap", (&OpsConfig{SnapshotDir: " /var/snap "}).SnapshotDirOrDefault())
}
