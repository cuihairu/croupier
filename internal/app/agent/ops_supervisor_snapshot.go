package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// 崩溃快照（S3，依 crash-capture 调研分级方案）：
//   - 轻档=spawn 注入语言原生开关（snapshotProfile 闭集 none/go/node/python/jvm），
//     产物落 per-process 快照目录（0700，含进程内存属敏感数据）；
//   - 中档=core_pattern 兜底，agent 只读提示（扫描宿主机现状+建议命令，
//     不直接写宿主配置——权限/安全边界）；
//   - 保留期/配额/下载权限未拍板，本批不实现（产物登记进事件，下载链留位）。

// snapshotProfileNone 是缺省档（不注入任何开关）。
const snapshotProfileNone = "none"

// snapshotEventHint 是 core_pattern 只读提示事件（宿主机级，process 为空）。
const snapshotEventHint = "snapshot_hint"

// snapshotDirMode 是快照目录权限（产物含进程内存明文，收紧 ACL）。
const snapshotDirMode os.FileMode = 0o700

// snapshotReportMax 是 node 报告在 WorkingDir 中登记的上限（防目录膨胀）。
const snapshotReportMax = 5

// snapshotNodeReportGlob 是 node --report-on-fatalerror 产物文件名模式。
const snapshotNodeReportGlob = "report*.json"

// snapshotNodeArgs 是 node 档注入的崩溃报告开关（NODE_OPTIONS 不允许
// 这两个 flag，只能走 argv；已存在则不重复注入）。
var snapshotNodeArgs = []string{"--report-on-fatalerror", "--report-exclude-env"}

// snapshotEnv 返回轻档按 profile 注入的环境变量（K=V 形式）。
// 调用方把返回值追加在 os.Environ() 与用户 env 之后（exec 去重取最后值）。
func snapshotEnv(cfg ManagedProcessConfig, snapDir string) []string {
	switch cfg.SnapshotProfile {
	case "go":
		// GOTRACEBACK=crash：崩溃时全 goroutine 栈转储并尝试 core dump
		// （core 落盘位置取决于宿主机 core_pattern，即中档）。
		return []string{"GOTRACEBACK=crash"}
	case "python":
		// faulthandler 把 Python 栈打到 stderr（进程日志侧可见）。
		return []string{"PYTHONFAULTHANDLER=1"}
	case "jvm":
		// JAVA_TOOL_OPTIONS 被 JVM 自动读取；HeapDumpPath 指向快照目录。
		// 已知副作用：JVM 启动时向 stderr 打印 "Picked up JAVA_TOOL_OPTIONS"。
		return []string{"JAVA_TOOL_OPTIONS=-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=" + snapDir}
	default:
		return nil
	}
}

// snapshotArgs 返回轻档按 profile 注入的 argv 前置参数（仅 node 档需要）。
func snapshotArgs(cfg ManagedProcessConfig) []string {
	if cfg.SnapshotProfile != "node" {
		return nil
	}
	return snapshotNodeArgs
}

// applySnapshotProfile 把轻档开关注入 cmd（env 追加在调用方 env 之后——
// exec 对重复 key 取最后值，显式声明的档位优先；node 档另做 argv 前置）。
func applySnapshotProfile(cmd *exec.Cmd, cfg ManagedProcessConfig, snapDir string) {
	cmd.Env = append(cmd.Env, snapshotEnv(cfg, snapDir)...)
	extra := snapshotArgs(cfg)
	if len(extra) == 0 || len(cmd.Args) == 0 {
		return
	}
	for _, a := range extra {
		if argHas(cmd.Args, a) {
			return
		}
	}
	cmd.Args = append([]string{cmd.Args[0]}, append(append([]string{}, extra...), cmd.Args[1:]...)...)
}

func argHas(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// ensureProcessSnapshotDir 创建 per-process 快照目录（0700），返回路径。
// 目录创建失败不阻断启动（env 档开关仍生效，只是没有落盘位置）。
func ensureProcessSnapshotDir(base, name string) string {
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, snapshotDirMode); err != nil {
		return ""
	}
	return dir
}

// readCorePattern 只读扫描宿主机 core_pattern 现状（中档前提）。
// 非 Linux 或读取失败返回空串——best-effort，不阻断监管。
// 包级变量以便测试注入确定性值。
var readCorePattern = func() string {
	raw, err := os.ReadFile("/proc/sys/kernel/core_pattern")
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(raw), "\x00\n")
}

// corePatternPipeMode 报告 core_pattern 是否为管道模式（"|..." = 交给
// systemd-coredump 或自定义 handler，中档已可用）。
func corePatternPipeMode(v string) bool {
	return strings.HasPrefix(v, "|")
}

// corePatternSuggestedCommand 是中档的建议命令（供运维一键复制，agent
// 只展示不执行——宿主机全局资源，写它影响面超出 agent）。
func corePatternSuggestedCommand() string {
	return "sudo sysctl -w kernel.core_pattern='|/usr/lib/systemd/systemd-coredump %P %u %g %s %t %c %e'"
}

// snapshotHintNeeded 报告是否需要发只读提示：有进程开了轻档且宿主机
// core_pattern 非管道模式（中档不可用，core 兜底缺失）。
func snapshotHintNeeded(corePattern string, anyProfileEnabled bool) bool {
	return anyProfileEnabled && corePattern != "" && !corePatternPipeMode(corePattern)
}

// snapshotHintEvent 构造宿主机级只读提示事件（process 为空）。
func snapshotHintEvent(corePattern string) *opsv1.SupervisorEvent {
	ev := newSupervisorEvent("", snapshotEventHint)
	ev.Message = fmt.Sprintf(
		"宿主机 core_pattern=%q 非管道模式，core 兜底不可用；如需中档快照请执行（agent 不会自动修改宿主配置）：%s",
		corePattern, corePatternSuggestedCommand())
	return ev
}

// collectSnapshotArtifacts 登记崩溃后可用的快照产物文件名（仅文件名，
// 路径由调用方结合 snapshot_dir 还原）。扫描面有界：快照目录全量 +
// node 报告在 WorkingDir 中按 mtime 倒序取最新 snapshotReportMax 个。
func collectSnapshotArtifacts(snapDir, workDir, profile string) []string {
	if snapDir == "" {
		return nil
	}
	var files []string
	if entries, err := os.ReadDir(snapDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			files = append(files, e.Name())
		}
	}
	if profile == "node" && workDir != "" {
		files = append(files, nodeReportFiles(workDir)...)
	}
	sort.Strings(files)
	return files
}

// nodeReportFiles 返回 WorkingDir 中最新的至多 snapshotReportMax 个报告
// 文件名（mtime 倒序）。node 报告默认落进程 cwd。
func nodeReportFiles(workDir string) []string {
	entries, err := filepath.Glob(filepath.Join(workDir, snapshotNodeReportGlob))
	if err != nil || len(entries) == 0 {
		return nil
	}
	type namedMod struct {
		name string
		mod  time.Time
	}
	var mods []namedMod
	for _, p := range entries {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			mods = append(mods, namedMod{name: filepath.Base(p), mod: fi.ModTime()})
		}
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].mod.After(mods[j].mod) })
	if len(mods) > snapshotReportMax {
		mods = mods[:snapshotReportMax]
	}
	out := make([]string, 0, len(mods))
	for _, m := range mods {
		out = append(out, m.name)
	}
	return out
}

// snapshotProfileEnabled 报告快照档是否开启（非 none/空）。
func snapshotProfileEnabled(v string) bool {
	return v != "" && v != snapshotProfileNone
}

// anySnapshotProfileEnabled 报告是否有进程开了轻档（非 none/空）。
func anySnapshotProfileEnabled(processes map[string]ManagedProcessConfig) bool {
	for _, cfg := range processes {
		if snapshotProfileEnabled(cfg.SnapshotProfile) {
			return true
		}
	}
	return false
}

// emitSnapshotHintOnce 在 agent 启动/首次手工启动进程时发一次 core_pattern
// 只读提示（once 保证不刷屏）。
func (s *OpsServer) emitSnapshotHintOnce() {
	s.hintOnce.Do(func() {
		cp := readCorePattern()
		if !snapshotHintNeeded(cp, anySnapshotProfileEnabled(s.config.ManagedProcesses)) {
			return
		}
		s.supLog.emit(snapshotHintEvent(cp))
	})
}
