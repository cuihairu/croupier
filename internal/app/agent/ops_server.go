// Package agent provides Ops server implementation.
// This replaces the gRPC-based Ops server with a lightweight implementation.
package agent

import (
	"os/exec"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	processcore "github.com/cuihairu/croupier/core/process"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func restartConfig(config ManagedProcessConfig) processcore.RestartConfig {
	return processcore.RestartConfig{
		InitialBackoff: config.RestartBackoffInitial,
		MaxBackoff:     config.RestartBackoffMax,
		BreakerLimit:   config.RestartBreakerLimit,
	}
}

// OpsServer implements ops functionality for the agent.
// It provides system info, process management, and command execution capabilities.
type OpsServer struct {
	mu      sync.RWMutex
	config  *OpsConfig
	agentID string
	version string

	// Managed process tracking
	processes map[string]*managedProcess

	// supLog 是监管事件环+轮转文件（S2 双通道的 agent 侧 truth）。
	supLog *supervisorEventLog

	// desired 受管进程管理状态（§10.3 管理状态面）：运维手动 start/stop 的
	// 期望跨 agent 重启保留，开机自启不再误启被主动停掉的进程。
	desired *desiredStateStore

	// schedPause 调度暂停状态（§10.2 暂停与恢复，暂停对象=调度）：运维维护
	// 窗口的 cron 冻结意图，跨 agent 重启保留；scheduleTick 跳过暂停进程。
	schedPause *schedulePauseStore

	// 调度状态（§10.6）：scheds 构造期编译（config 构造后不变，快照与循环
	// 共读）；schedStop 由 Stop 关闭；schedFired 只属调度循环 goroutine；
	// schedTick 缺省 30s（测试钩子，直接驱动 scheduleTick 时拉长）。
	scheds     map[string][]processScheduleEntry
	schedStop  chan struct{}
	schedFired map[string]string
	schedTick  time.Duration

	// updateStabilityWindow 更新稳定性窗口（§10.4）：缺省 5s（零值走缺省常量，
	// 测试注入短窗口）。
	updateStabilityWindow time.Duration

	// hintOnce 保证 core_pattern 只读提示（S3 中档）只发一次。
	hintOnce sync.Once
}

// managedProcess represents a managed process state
type managedProcess struct {
	name      string
	cmd       *exec.Cmd
	state     opsv1.ProcessState
	pid       int32
	restarts  int32
	lastStart *timestamppb.Timestamp
	config    ManagedProcessConfig
	stopCh    chan struct{}
	// waitDone 由 monitorProcess 在 cmd.Wait() 返回后关闭——
	// Wait 的唯一属主是 monitorProcess；stopProcess 杀进程后等它收尸，
	// 消除并发双 Wait 的 DATA RACE。
	waitDone chan struct{}
	// supervisor 采样缓存与 gopsutil 句柄（CPUPercent 依赖同一句柄做
	// 区间差分）。均由 p.mu 保护。
	proc      *process.Process
	lastRSS   int64
	lastCPU   float64
	lastFlags []string
	// S2 监管状态机字段（p.mu 保护）：consecutiveFails 是连续失败计数
	// （存活超过退避封顶即清零）；nextRestartAt 是 BACKOFF 态的下次拉起
	// 时刻；lastEventUnix/lastSampleUnix 供快照的 last_event_unix 与
	// 事件上下文 last_heartbeat 使用。
	consecutiveFails int
	nextRestartAt    *time.Time
	supervisor       *processcore.SupervisorState
	lastEventUnix    int64
	lastSampleUnix   int64
	// snapshotDir 是本实例的崩溃快照目录（S3 轻档，startProcess 时建），
	// 建目录失败为空串（env 档开关仍注入，只是无落盘位置）。
	snapshotDir string
	// fingerprint 是本实例启动时二进制的 sha256（§10.4 指纹硬要求）。spawn 时
	// 算一次并缓存：运行中实例仍跑旧 inode，文件被换走后本字段仍反映进程实际
	// 运行的内容；空串=未运行或内容读不到（未运行实例的快照字段为空）。
	fingerprint string
	mu          sync.RWMutex
}

// NewOpsServer creates a new Ops server instance.
func NewOpsServer(config *OpsConfig, agentID, version string, _ interface{}) *OpsServer {
	if config == nil {
		config = DefaultOpsConfig()
	}
	return &OpsServer{
		config:     config,
		agentID:    agentID,
		version:    version,
		processes:  make(map[string]*managedProcess),
		supLog:     newSupervisorEventLog(config.SupervisorLog),
		desired:    newDesiredStateStore(config.SupervisorLog),
		scheds:     compileProcessSchedules(config.ManagedProcesses),
		schedPause: newSchedulePauseStore(config.SupervisorLog),
	}
}
