// Package agent provides Ops server implementation for NNG communication.
// This replaces the gRPC-based Ops server with a lightweight implementation
// that can be wrapped for NNG AgentServer.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OpsServer implements ops functionality for the agent.
// It provides system info, process management, and command execution capabilities.
type OpsServer struct {
	mu      sync.RWMutex
	config  *OpsConfig
	agentID string
	version string

	// Managed process tracking
	processes map[string]*managedProcess
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
	mu        sync.RWMutex
}

// NewOpsServer creates a new Ops server instance.
func NewOpsServer(config *OpsConfig, agentID, version string, _ interface{}) *OpsServer {
	if config == nil {
		config = DefaultOpsConfig()
	}
	return &OpsServer{
		config:    config,
		agentID:   agentID,
		version:   version,
		processes: make(map[string]*managedProcess),
	}
}

// GetSystemInfo returns system information.
func (s *OpsServer) GetSystemInfo(ctx context.Context, _ *emptypb.Empty) (*opsv1.SystemInfo, error) {
	return GetSystemInfo(s.agentID, s.version, s.config), nil
}

// ListProcesses returns the list of managed processes.
func (s *OpsServer) ListProcesses(ctx context.Context, _ *emptypb.Empty) (*opsv1.ListProcessesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := &opsv1.ListProcessesResponse{
		Processes: make([]*opsv1.ManagedProcess, 0, len(s.processes)),
	}

	for _, p := range s.processes {
		p.mu.RLock()
		mp := &opsv1.ManagedProcess{
			Name:         p.name,
			Command:      p.config.Command,
			WorkingDir:   p.config.WorkingDir,
			State:        p.state,
			Pid:          p.pid,
			RestartCount: p.restarts,
			LastStart:    p.lastStart,
		}
		p.mu.RUnlock()
		resp.Processes = append(resp.Processes, mp)
	}

	return resp, nil
}

// ReportMetrics handles metrics reporting (just acknowledges).
func (s *OpsServer) ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error) {
	// Metrics are handled by the MetricsCollector in upstream.go
	// This is just an acknowledgment for NNG protocol
	return &emptypb.Empty{}, nil
}

// RestartProcess restarts a managed process.
func (s *OpsServer) RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error) {
	if !s.config.Enabled || !s.config.AllowRestart {
		return nil, fmt.Errorf("ops restart is not enabled")
	}

	s.mu.Lock()
	p, ok := s.processes[req.ProcessName]
	s.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("process '%s' not found", req.ProcessName)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop the process
	s.stopProcess(p)

	// Start it again
	if err := s.startProcess(p); err != nil {
		return nil, fmt.Errorf("failed to restart process: %w", err)
	}

	p.restarts++
	p.lastStart = timestamppb.Now()
	p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING

	return &opsv1.RestartProcessResponse{
		Success: true,
		Message: fmt.Sprintf("Process '%s' restarted", req.ProcessName),
	}, nil
}

// StopProcess stops a managed process.
func (s *OpsServer) StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error) {
	if !s.config.Enabled || !s.config.AllowRestart {
		return nil, fmt.Errorf("ops restart is not enabled")
	}

	s.mu.Lock()
	p, ok := s.processes[req.ProcessName]
	s.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("process '%s' not found", req.ProcessName)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	s.stopProcess(p)
	p.state = opsv1.ProcessState_PROCESS_STATE_STOPPED

	return &opsv1.StopProcessResponse{
		Success: true,
		Message: fmt.Sprintf("Process '%s' stopped", req.ProcessName),
	}, nil
}

// StartProcess starts a managed process.
func (s *OpsServer) StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error) {
	if !s.config.Enabled || !s.config.AllowRestart {
		return nil, fmt.Errorf("ops restart is not enabled")
	}

	s.mu.Lock()
	p, ok := s.processes[req.ProcessName]
	s.mu.Unlock()

	if !ok {
		// Check if we have a config for this process
		s.mu.RLock()
		cfg, hasCfg := s.config.ManagedProcesses[req.ProcessName]
		s.mu.RUnlock()

		if !hasCfg {
			return nil, fmt.Errorf("process '%s' not configured", req.ProcessName)
		}

		// Create new managed process
		p = &managedProcess{
			name:   req.ProcessName,
			config: cfg,
			state:  opsv1.ProcessState_PROCESS_STATE_STOPPED,
			stopCh: make(chan struct{}),
		}

		s.mu.Lock()
		s.processes[req.ProcessName] = p
		s.mu.Unlock()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == opsv1.ProcessState_PROCESS_STATE_RUNNING {
		return nil, fmt.Errorf("process '%s' is already running", req.ProcessName)
	}

	if err := s.startProcess(p); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	p.restarts++
	p.lastStart = timestamppb.Now()
	p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING

	return &opsv1.StartProcessResponse{
		Success: true,
		Message: fmt.Sprintf("Process '%s' started", req.ProcessName),
		Pid:     p.pid,
	}, nil
}

// ExecuteCommand executes a command on the agent.
func (s *OpsServer) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	if !s.config.Enabled || !s.config.AllowExec {
		return nil, fmt.Errorf("ops exec is not enabled")
	}

	// Check if command is allowed
	if len(s.config.ExecAllowedCommands) > 0 {
		allowed := false
		for _, cmd := range s.config.ExecAllowedCommands {
			if cmd == req.Command {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("command '%s' is not allowed", req.Command)
		}
	}

	// Create command with timeout
	timeout := s.config.ExecTimeout
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
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

	// Set environment variables if provided
	if len(req.Env) > 0 {
		env := os.Environ()
		for k, v := range req.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	output, err := cmd.CombinedOutput()

	exitCode := int32(0)
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitError.ExitCode())
		} else {
			exitCode = -1
		}
	}

	return &opsv1.ExecuteCommandResponse{
		ExitCode: exitCode,
		StdOut:   string(output),
		StdErr:   "", // Combined in stdout
	}, nil
}

// startProcess starts a managed process
func (s *OpsServer) startProcess(p *managedProcess) error {
	cmd := exec.Command(p.config.Command, p.config.Args...)
	if p.config.WorkingDir != "" {
		cmd.Dir = p.config.WorkingDir
	}

	// Set environment variables
	env := os.Environ()
	for k, v := range p.config.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	p.cmd = cmd
	p.pid = int32(cmd.Process.Pid)

	// Start goroutine to monitor process
	go s.monitorProcess(p)

	return nil
}

// stopProcess stops a managed process
func (s *OpsServer) stopProcess(p *managedProcess) {
	if p.stopCh != nil {
		select {
		case <-p.stopCh:
			// Already closed
		default:
			close(p.stopCh)
		}
	}

	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}
}

// monitorProcess monitors a managed process and restarts if needed
func (s *OpsServer) monitorProcess(p *managedProcess) {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}

	err := p.cmd.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != opsv1.ProcessState_PROCESS_STATE_STOPPED {
		if p.config.AutoRestart && err != nil {
			// Restart the process
			delay := p.config.RestartDelay
			if delay <= 0 {
				delay = 5 * time.Second
			}
			time.Sleep(delay)
			if s.startProcess(p) == nil {
				p.restarts++
				p.lastStart = timestamppb.Now()
			}
		} else {
			p.state = opsv1.ProcessState_PROCESS_STATE_FAILED
		}
	}
}

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

		p := &managedProcess{
			name:   name,
			config: cfg,
			state:  opsv1.ProcessState_PROCESS_STATE_STOPPED,
			stopCh: make(chan struct{}),
		}

		if err := s.startProcess(p); err != nil {
			// Log but don't fail
			continue
		}

		p.restarts = 1
		p.lastStart = timestamppb.Now()
		p.state = opsv1.ProcessState_PROCESS_STATE_RUNNING
		s.processes[name] = p
	}

	return nil
}

// Stop stops all managed processes.
func (s *OpsServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range s.processes {
		p.mu.Lock()
		s.stopProcess(p)
		p.state = opsv1.ProcessState_PROCESS_STATE_STOPPED
		p.mu.RUnlock()
	}
}

// Close stops the Ops server.
func (s *OpsServer) Close() error {
	s.Stop()
	return nil
}

// ========== System Services (JSON methods to avoid circular dependency) ==========

// ListServicesJSON handles ListServicesRequest via JSON
func (s *OpsServer) ListServicesJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	var req ListServicesRequest
	if len(jsonReq) > 0 {
		if err := json.Unmarshal(jsonReq, &req); err != nil {
			return nil, err
		}
	}

	services, err := ListServices(req.State, req.NamePattern, int(req.Limit))
	if err != nil {
		return nil, err
	}

	// Return JSON response
	resp := struct {
		Services []*ServiceInfo `json:"services"`
		Total    int32          `json:"total"`
	}{
		Services: make([]*ServiceInfo, len(services)),
		Total:    int32(len(services)),
	}
	for i := range services {
		resp.Services[i] = &services[i]
	}

	return json.Marshal(resp)
}

// GetServiceStatusJSON handles GetServiceStatusRequest via JSON
func (s *OpsServer) GetServiceStatusJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	var req GetServiceStatusRequest
	if err := json.Unmarshal(jsonReq, &req); err != nil {
		return nil, err
	}

	status, err := GetServiceStatus(req.Name)
	if err != nil {
		return nil, err
	}

	// Return JSON response
	resp := &ServiceStatusDetail{
		Name:        status.Name,
		DisplayName: status.DisplayName,
		Status:      status.Status,
		StartType:   status.StartType,
		ProcessID:   status.ProcessID,
		BinaryPath:  status.BinaryPath,
		Description: status.Description,
	}

	return json.Marshal(resp)
}

// ListCronJobsJSON handles ListCronJobsRequest via JSON
func (s *OpsServer) ListCronJobsJSON(ctx context.Context) ([]byte, error) {
	jobs, err := ListCronJobs()
	if err != nil {
		return nil, err
	}

	// Return JSON response
	resp := struct {
		Jobs  []*CronJob `json:"jobs"`
		Total int32      `json:"total"`
	}{
		Jobs:  make([]*CronJob, len(jobs)),
		Total: int32(len(jobs)),
	}
	for i := range jobs {
		resp.Jobs[i] = &jobs[i]
	}

	return json.Marshal(resp)
}

// ========== System Services ==========

// ListServicesRequest requests a list of system services.
type ListServicesRequest struct {
	State       string `json:"state"`
	NamePattern string `json:"name_pattern"`
	Limit       int32  `json:"limit"`
}

// ListServicesResponse contains the list of system services.
type ListServicesResponse struct {
	Services []*ServiceInfo `json:"services"`
	Total    int32          `json:"total"`
}

// GetServiceStatusRequest requests detailed status of a specific service.
type GetServiceStatusRequest struct {
	Name string `json:"name"`
}

// GetServiceStatusResponse contains detailed service status.
type GetServiceStatusResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	StartType   string `json:"start_type"`
	ProcessID   uint32 `json:"process_id"`
	BinaryPath  string `json:"binary_path"`
	Description string `json:"description"`
}

// ListCronJobsResponse contains the list of cron jobs.
type ListCronJobsResponse struct {
	Jobs  []*CronJob `json:"jobs"`
	Total int32      `json:"total"`
}

// ListServices returns system services.
func (s *OpsServer) ListServices(ctx context.Context, req *ListServicesRequest) (*ListServicesResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	services, err := ListServices(req.State, req.NamePattern, limit)
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	servicePtrs := make([]*ServiceInfo, len(services))
	for i := range services {
		servicePtrs[i] = &services[i]
	}

	return &ListServicesResponse{
		Services: servicePtrs,
		Total:    int32(len(services)),
	}, nil
}

// GetServiceStatus returns detailed service status.
func (s *OpsServer) GetServiceStatus(ctx context.Context, req *GetServiceStatusRequest) (*GetServiceStatusResponse, error) {
	status, err := GetServiceStatus(req.Name)
	if err != nil {
		return nil, err
	}

	return &GetServiceStatusResponse{
		Name:        status.Name,
		DisplayName: status.DisplayName,
		Status:      status.Status,
		StartType:   status.StartType,
		ProcessID:   status.ProcessID,
		BinaryPath:  status.BinaryPath,
		Description: status.Description,
	}, nil
}

// ListCronJobs returns cron jobs on Linux systems.
func (s *OpsServer) ListCronJobs(ctx context.Context) (*ListCronJobsResponse, error) {
	jobs, err := ListCronJobs()
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	jobPtrs := make([]*CronJob, len(jobs))
	for i := range jobs {
		jobPtrs[i] = &jobs[i]
	}

	return &ListCronJobsResponse{
		Jobs:  jobPtrs,
		Total: int32(len(jobs)),
	}, nil
}
