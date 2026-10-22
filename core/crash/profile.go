// Package crash 崩溃采集轻档公共件（agent-core K4，分级依据见
// docs/research/crash-capture-survey-2026-10.md：轻=spawn 注入语言原生开关
// 为默认、中=OS 层 core_pattern 兜底可选、重=Crashpad 否决不立项）。
//
// 职责边界：本包只提供 ①snapshot 档位 → spawn 注入物（env/args）的映射，
// ②agent 管理的 dump 产物目录（列表/保留期/配额）。与 supervisor 的
// ManagedProcesses 接线（snapshotProfile 字段、下载面板）随 #67 S3 落地；
// 轻档显式声明档位、不做语言自动探测——误判的代价是注入无效或污染环境。
package crash

import "fmt"

// Profile 快照档位（显式声明，随进程配置，不自动探测语言）。
type Profile string

const (
	ProfileNone   Profile = "none"
	ProfileGo     Profile = "go"
	ProfileNode   Profile = "node"
	ProfilePython Profile = "python"
	ProfileJVM    Profile = "jvm"
)

// Injection 一次轻档注入物：spawn 前并入进程 env 与 args。
type Injection struct {
	Env  []string // KEY=VALUE 形态
	Args []string // 追加到命令行尾部
}

// Inject 返回该档位的 spawn 注入物。dumpDir 为 agent 管理的快照落盘目录
// （JVM HeapDumpPath / Node report 目录语义）。none 返回空注入；未知档位
// 报错（宁拒绝不猜语言）。
func Inject(profile Profile, dumpDir string) (Injection, error) {
	switch profile {
	case ProfileNone, "":
		return Injection{}, nil
	case ProfileGo:
		// GOTRACEBACK=crash：panic 时触发 core dump（配合中档 core_pattern
		// 才有产物）并保留完整 goroutine 栈输出。
		return Injection{Env: []string{"GOTRACEBACK=crash"}}, nil
	case ProfileNode:
		// --report-exclude-env 强制：Diagnostic Report 默认内嵌环境变量，
		// 常含密钥（调研 §7 敏感数据纪律）。
		if dumpDir == "" {
			return Injection{}, fmt.Errorf("crash: node profile requires dumpDir")
		}
		return Injection{Args: []string{
			"--report-on-fatalerror",
			"--report-exclude-env",
			"--report-directory=" + dumpDir,
		}}, nil
	case ProfilePython:
		// PYTHONFAULTHANDLER=1：致命错误时把 Python 栈转储到 stderr。
		return Injection{Env: []string{"PYTHONFAULTHANDLER=1"}}, nil
	case ProfileJVM:
		if dumpDir == "" {
			return Injection{}, fmt.Errorf("crash: jvm profile requires dumpDir")
		}
		return Injection{Args: []string{
			"-XX:+HeapDumpOnOutOfMemoryError",
			"-XX:HeapDumpPath=" + dumpDir,
		}}, nil
	default:
		return Injection{}, fmt.Errorf("crash: unknown snapshot profile %q", profile)
	}
}
