package agent

import (
	"context"
	"os"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Start starts all configured managed processes.
func (s *OpsServer) Start() error {
	if !s.config.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for name, cfg := range s.config.ManagedProcesses {
		if !cfg.AutoRestart {
			continue
		}
		// 管理状态门（§10.2 区分主动停止）：期望无记录沿用既有语义
		// （AutoRestart 即拉起）；运维手动停过的进程跨 agent 重启保持停。
		if !s.desired.bootShouldStart(name) {
			continue
		}

		p := newManagedProcess(name, cfg)

		if err := s.startProcess(p); err != nil {
			// Log but don't fail
			continue
		}

		// monitor goroutine 已随 startProcess 启动：实例字段写入必须过
		// p.mu（s.mu 管不了 monitor 与 superviseExit 的读路径）。
		p.mu.Lock()
		p.restarts = 1
		p.lastStart = timestamppb.Now()
		p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
		p.mu.Unlock()
		s.processes[name] = p
	}

	// S3 中档只读提示：有进程开轻档且宿主机 core_pattern 非管道模式时发
	// 一次建议命令（once 保证不刷屏；agent 不写宿主配置）。
	s.emitSnapshotHintOnce()

	// §10.6 调度状态：有 schedule 声明才起分钟槽扫描循环（零配置零漂移）。
	s.startScheduleLoopLocked()

	return nil
}

// GetSupervisorLog returns a tail of the rotating supervisor event log for
// the download proxy (server pulls via the ops tunnel; size-capped).
func (s *OpsServer) GetSupervisorLog(ctx context.Context, req *opsv1.GetSupervisorLogRequest) (*opsv1.GetSupervisorLogResponse, error) {
	content, fileName, truncated, err := s.supLog.ReadTail(int64(req.GetMaxBytes()))
	if err != nil {
		if os.IsNotExist(err) {
			// 尚未产生事件时文件不存在：返回空内容而非错误，面板可直接下载空日志。
			return &opsv1.GetSupervisorLogResponse{Content: []byte{}, FileName: fileName}, nil
		}
		return nil, err
	}
	return &opsv1.GetSupervisorLogResponse{
		Content:   content,
		FileName:  fileName,
		Truncated: truncated,
	}, nil
}

// Stop stops all managed processes.
func (s *OpsServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 调度循环先停：Stop 之后的进程收尾不该再被定时动作打断。
	if s.schedStop != nil {
		close(s.schedStop)
		s.schedStop = nil
	}

	for _, p := range s.processes {
		p.mu.Lock()
		s.stopProcess(p)
		p.state = opsv1.ProcessState_PROCESS_STATE_STOPPED
		// 此前误用 RUnlock：写锁配读解锁，processes 非空时 Go 运行时
		// 直接 fatal "sync: RUnlock of unlocked RWMutex" 崩溃整个 agent。
		p.mu.Unlock()
	}
}

// Close stops the Ops server.
func (s *OpsServer) Close() error {
	s.Stop()
	return nil
}

// ========== System Services (JSON methods to avoid circular dependency) ==========
