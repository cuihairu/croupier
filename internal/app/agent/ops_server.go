package agent

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OpsServer implements the OpsService gRPC server.
type OpsServer struct {
	opsv1.UnimplementedOpsServiceServer

	config       *OpsConfig
	agentID      string
	agentVersion string
	logger       *slog.Logger

	// Process management
	processMu sync.RWMutex
	processes map[string]*managedProcess
}

type managedProcess struct {
	config       ManagedProcessConfig
	cmd          *exec.Cmd
	state        opsv1.ProcessState
	pid          int
	restartCount int
	lastStart    time.Time
	mu           sync.Mutex
}

// NewOpsServer creates a new OpsServer.
func NewOpsServer(config *OpsConfig, agentID, agentVersion string, logger *slog.Logger) *OpsServer {
	if config == nil {
		config = DefaultOpsConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	s := &OpsServer{
		config:       config,
		agentID:      agentID,
		agentVersion: agentVersion,
		logger:       logger.With("component", "ops"),
		processes:    make(map[string]*managedProcess),
	}

	// Initialize managed processes from config
	for name, cfg := range config.ManagedProcesses {
		s.processes[name] = &managedProcess{
			config: cfg,
			state:  opsv1.ProcessState_PROCESS_STATE_STOPPED,
		}
	}

	return s
}

// ReportMetrics receives metrics from client (not typically used - agent reports to server).
// Note: This method does NOT require ops.enabled - metrics are always allowed.
func (s *OpsServer) ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error) {
	s.logger.Info("received metrics report",
		"agent_id", req.AgentId,
		"cpu_usage", req.GetCpu().GetUsagePercent(),
		"memory_usage", req.GetMemory().GetUsagePercent(),
	)

	return &emptypb.Empty{}, nil
}

// StreamMetrics receives streaming metrics.
// Note: This method does NOT require ops.enabled - metrics are always allowed.
func (s *OpsServer) StreamMetrics(stream opsv1.OpsService_StreamMetricsServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		s.logger.Debug("received streaming metrics",
			"agent_id", req.AgentId,
			"cpu_usage", req.GetCpu().GetUsagePercent(),
		)
	}
}

// GetSystemInfo returns system information.
// Note: This method does NOT require ops.enabled - system info is always allowed.
func (s *OpsServer) GetSystemInfo(ctx context.Context, _ *emptypb.Empty) (*opsv1.SystemInfo, error) {
	return GetSystemInfo(s.agentID, s.agentVersion, s.config), nil
}

// RestartProcess restarts a managed process.
func (s *OpsServer) RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error) {
	if !s.config.Enabled {
		return nil, status.Error(codes.Unavailable, "ops module is disabled")
	}
	if !s.config.AllowRestart {
		return nil, status.Error(codes.PermissionDenied, "restart operations are not allowed")
	}

	s.logger.Warn("restart process requested",
		"process", req.ProcessName,
		"force", req.Force,
	)

	s.processMu.RLock()
	proc, exists := s.processes[req.ProcessName]
	s.processMu.RUnlock()

	if !exists {
		return &opsv1.RestartProcessResponse{
			Success: false,
			Message: fmt.Sprintf("process %q not found in managed processes", req.ProcessName),
		}, nil
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	// Stop existing process if running
	if proc.cmd != nil && proc.cmd.Process != nil {
		timeout := time.Duration(req.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = proc.config.GracefulTimeout
		}
		if timeout <= 0 {
			timeout = 30 * time.Second
		}

		if req.Force {
			proc.cmd.Process.Kill()
		} else {
			proc.cmd.Process.Signal(os.Interrupt)
			done := make(chan error, 1)
			go func() { done <- proc.cmd.Wait() }()
			select {
			case <-done:
			case <-time.After(timeout):
				proc.cmd.Process.Kill()
			}
		}
	}

	// Start new process
	cmd := exec.CommandContext(ctx, proc.config.Command, proc.config.Args...)
	if proc.config.WorkingDir != "" {
		cmd.Dir = proc.config.WorkingDir
	}
	for k, v := range proc.config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Start(); err != nil {
		return &opsv1.RestartProcessResponse{
			Success: false,
			Message: fmt.Sprintf("failed to start process: %v", err),
		}, nil
	}

	proc.cmd = cmd
	proc.pid = cmd.Process.Pid
	proc.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
	proc.restartCount++
	proc.lastStart = time.Now()

	s.logger.Info("process restarted",
		"process", req.ProcessName,
		"pid", proc.pid,
		"restart_count", proc.restartCount,
	)

	return &opsv1.RestartProcessResponse{
		Success: true,
		Message: "process restarted successfully",
		NewPid:  int32(proc.pid),
	}, nil
}

// StopProcess stops a managed process.
func (s *OpsServer) StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error) {
	if !s.config.Enabled {
		return nil, status.Error(codes.Unavailable, "ops module is disabled")
	}
	if !s.config.AllowRestart {
		return nil, status.Error(codes.PermissionDenied, "stop operations are not allowed")
	}

	s.logger.Warn("stop process requested",
		"process", req.ProcessName,
		"force", req.Force,
	)

	s.processMu.RLock()
	proc, exists := s.processes[req.ProcessName]
	s.processMu.RUnlock()

	if !exists {
		return &opsv1.StopProcessResponse{
			Success: false,
			Message: fmt.Sprintf("process %q not found", req.ProcessName),
		}, nil
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	if proc.cmd == nil || proc.cmd.Process == nil {
		return &opsv1.StopProcessResponse{
			Success: true,
			Message: "process is not running",
		}, nil
	}

	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = proc.config.GracefulTimeout
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if req.Force {
		proc.cmd.Process.Kill()
	} else {
		proc.cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- proc.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(timeout):
			proc.cmd.Process.Kill()
		}
	}

	proc.state = opsv1.ProcessState_PROCESS_STATE_STOPPED
	proc.pid = 0

	s.logger.Info("process stopped", "process", req.ProcessName)

	return &opsv1.StopProcessResponse{
		Success: true,
		Message: "process stopped",
	}, nil
}

// StartProcess starts a managed process.
func (s *OpsServer) StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error) {
	if !s.config.Enabled {
		return nil, status.Error(codes.Unavailable, "ops module is disabled")
	}
	if !s.config.AllowRestart {
		return nil, status.Error(codes.PermissionDenied, "start operations are not allowed")
	}

	s.logger.Info("start process requested", "process", req.ProcessName)

	s.processMu.RLock()
	proc, exists := s.processes[req.ProcessName]
	s.processMu.RUnlock()

	if !exists {
		return &opsv1.StartProcessResponse{
			Success: false,
			Message: fmt.Sprintf("process %q not found", req.ProcessName),
		}, nil
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	if proc.cmd != nil && proc.cmd.Process != nil {
		// Check if still running
		if proc.cmd.ProcessState == nil || !proc.cmd.ProcessState.Exited() {
			return &opsv1.StartProcessResponse{
				Success: false,
				Message: "process is already running",
				Pid:     int32(proc.pid),
			}, nil
		}
	}

	cmd := exec.CommandContext(ctx, proc.config.Command, proc.config.Args...)
	if proc.config.WorkingDir != "" {
		cmd.Dir = proc.config.WorkingDir
	}
	for k, v := range proc.config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Start(); err != nil {
		return &opsv1.StartProcessResponse{
			Success: false,
			Message: fmt.Sprintf("failed to start: %v", err),
		}, nil
	}

	proc.cmd = cmd
	proc.pid = cmd.Process.Pid
	proc.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
	proc.lastStart = time.Now()

	s.logger.Info("process started",
		"process", req.ProcessName,
		"pid", proc.pid,
	)

	return &opsv1.StartProcessResponse{
		Success: true,
		Message: "process started",
		Pid:     int32(proc.pid),
	}, nil
}

// ListProcesses lists all managed processes.
func (s *OpsServer) ListProcesses(ctx context.Context, _ *emptypb.Empty) (*opsv1.ListProcessesResponse, error) {
	if !s.config.Enabled {
		return nil, status.Error(codes.Unavailable, "ops module is disabled")
	}

	s.processMu.RLock()
	defer s.processMu.RUnlock()

	procs := make([]*opsv1.ManagedProcess, 0, len(s.processes))
	for name, proc := range s.processes {
		proc.mu.Lock()
		mp := &opsv1.ManagedProcess{
			Name:         name,
			Command:      proc.config.Command,
			WorkingDir:   proc.config.WorkingDir,
			State:        proc.state,
			Pid:          int32(proc.pid),
			RestartCount: int32(proc.restartCount),
		}
		if !proc.lastStart.IsZero() {
			mp.LastStart = timestamppb.New(proc.lastStart)
		}
		proc.mu.Unlock()
		procs = append(procs, mp)
	}

	return &opsv1.ListProcessesResponse{Processes: procs}, nil
}

// ExecuteCommand executes a shell command.
// WARNING: This is a high-risk operation.
func (s *OpsServer) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	if !s.config.Enabled {
		return nil, status.Error(codes.Unavailable, "ops module is disabled")
	}
	if !s.config.AllowExec {
		return nil, status.Error(codes.PermissionDenied, "command execution is not allowed")
	}

	// Check allowed commands if configured
	if len(s.config.ExecAllowedCommands) > 0 {
		if !slices.Contains(s.config.ExecAllowedCommands, req.Command) {
			return nil, status.Errorf(codes.PermissionDenied, "command %q is not in allowed list", req.Command)
		}
	}

	s.logger.Warn("executing command",
		"command", req.Command,
		"args", req.Args,
		"working_dir", req.WorkingDir,
	)

	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = s.config.ExecTimeout
	}
	if timeout > 300*time.Second {
		timeout = 300 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	resp := &opsv1.ExecuteCommandResponse{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			resp.Success = false
			resp.ExitCode = int32(exitErr.ExitCode())
		} else {
			resp.Success = false
			resp.Error = err.Error()
			resp.ExitCode = -1
		}
	} else {
		resp.Success = true
		resp.ExitCode = 0
	}

	s.logger.Info("command executed",
		"command", req.Command,
		"success", resp.Success,
		"exit_code", resp.ExitCode,
	)

	return resp, nil
}
