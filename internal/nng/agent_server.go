// Package nng provides NNG-based server implementation for Agent local services
package nng

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// AgentServer implements an NNG-based agent server for local services.
// It replaces the gRPC InvokerService, OpsService, and LocalControlService.
type AgentServer struct {
	addr string
	sock mangos.Socket

	// Dependencies
	store           *agentlocal.LocalStore
	jobs            *jobIndex
	platformManager PlatformManager
	opsServer       OpsServerWrapper
	tlsCfg          *tlsutil.ClientTLSConfig

	// State
	mu      sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// Logging
	logger *slog.Logger
}

// PlatformManager is the interface for platform function calls
type PlatformManager interface {
	IsPlatformFunction(functionID string) bool
	Call(ctx context.Context, functionID string, request []byte) ([]byte, error)
}

// OpsServerWrapper wraps the OpsServer functionality
type OpsServerWrapper interface {
	GetSystemInfo(ctx context.Context, req *emptypb.Empty) (*opsv1.SystemInfo, error)
	ListProcesses(ctx context.Context, req *emptypb.Empty) (*opsv1.ListProcessesResponse, error)
	ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error)
	RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error)
	StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error)
	StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error)
	ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error)
}

// NewAgentServer creates a new NNG agent server
func NewAgentServer(addr string, store *agentlocal.LocalStore) *AgentServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &AgentServer{
		addr:   addr,
		store:  store,
		jobs:   newJobIndex(),
		ctx:    ctx,
		cancel: cancel,
		logger: slog.Default(),
	}
}

// SetPlatformManager sets the platform manager
func (s *AgentServer) SetPlatformManager(pm PlatformManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.platformManager = pm
}

// SetOpsServer sets the ops server wrapper
func (s *AgentServer) SetOpsServer(ops OpsServerWrapper) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opsServer = ops
}

// SetTLSConfig sets the TLS config for outbound connections
func (s *AgentServer) SetTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tlsCfg = cfg
}

// SetLogger sets the logger
func (s *AgentServer) SetLogger(logger *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
}

// Start starts the NNG agent server
func (s *AgentServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	// Create REP socket
	sock, err := rep.NewSocket()
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}

	// Configure options
	if err := sock.SetOption(mangos.OptionRecvDeadline, time.Second); err != nil {
		sock.Close()
		return fmt.Errorf("failed to set recv deadline: %w", err)
	}

	// Listen
	listenAddr := "tcp://" + s.addr
	if err := sock.Listen(listenAddr); err != nil {
		sock.Close()
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.sock = sock
	s.logger.Info("NNG Agent server started", "addr", s.addr)

	// Start serving
	go s.serve()

	return nil
}

// Stop stops the NNG agent server
func (s *AgentServer) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	s.cancel()

	if s.sock != nil {
		if err := s.sock.Close(); err != nil {
			s.logger.Error("failed to close socket", "error", err)
		}
		s.sock = nil
	}

	s.logger.Info("NNG Agent server stopped")
	return nil
}

// GetAddr returns the server address
func (s *AgentServer) GetAddr() string {
	return s.addr
}

// serve handles incoming requests
func (s *AgentServer) serve() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		if s.sock == nil {
			return
		}

		// Receive message
		msg, err := s.sock.RecvMsg()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			continue
		}

		// Parse protocol header from body prefix
		_, msgID, reqID, data, err := protocol.ParseMessageFromBody(msg.Body)
		msg.Free()
		if err != nil {
			s.logger.Error("failed to parse message", "error", err)
			continue
		}

		// Handle request based on message type
		respData, err := s.handleRequest(s.ctx, msgID, data)
		if err != nil {
			s.logger.Error("failed to handle request", "msgID", protocol.MsgIDString(msgID), "error", err)
			// Send error response
			respData = s.createErrorResponse(err)
		}

		// Create response with protocol header in body
		respMsgID := protocol.GetResponseMsgID(msgID)
		respBodyWithHeader := protocol.NewMessageBody(respMsgID, reqID, respData)

		respMsg := mangos.NewMessage(0)
		respMsg.Body = respBodyWithHeader

		if err := s.sock.SendMsg(respMsg); err != nil {
			s.logger.Error("failed to send response", "error", err)
		}
	}
}

// handleRequest dispatches request to appropriate handler
func (s *AgentServer) handleRequest(ctx context.Context, msgID uint32, data []byte) ([]byte, error) {
	switch msgID {
	// InvokerService
	case protocol.MsgInvokeRequest:
		return s.handleInvoke(ctx, data)
	case protocol.MsgStartJobRequest:
		return s.handleStartJob(ctx, data)
	case protocol.MsgCancelJobRequest:
		return s.handleCancelJob(ctx, data)

	// OpsService
	case protocol.MsgGetSystemInfoRequest:
		return s.handleGetSystemInfo(ctx, data)
	case protocol.MsgListProcessesRequest:
		return s.handleListProcesses(ctx, data)
	case protocol.MsgReportMetricsRequest:
		return s.handleReportMetrics(ctx, data)
	case protocol.MsgRestartProcessRequest:
		return s.handleRestartProcess(ctx, data)
	case protocol.MsgStopProcessRequest:
		return s.handleStopProcess(ctx, data)
	case protocol.MsgStartProcessRequest:
		return s.handleStartProcess(ctx, data)
	case protocol.MsgExecuteCommandRequest:
		return s.handleExecuteCommand(ctx, data)

	// LocalControlService
	case protocol.MsgRegisterLocalRequest:
		return s.handleRegisterLocal(ctx, data)
	case protocol.MsgHeartbeatLocalRequest:
		return s.handleHeartbeatLocal(ctx, data)
	case protocol.MsgListLocalRequest:
		return s.handleListLocal(ctx, data)

	default:
		return nil, fmt.Errorf("unknown message type: 0x%06X", msgID)
	}
}

// handleInvoke handles InvokeRequest
func (s *AgentServer) handleInvoke(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest: %w", err)
	}

	functionID := req.GetFunctionId()

	// Check if this is a platform function call
	if s.platformManager != nil && s.platformManager.IsPlatformFunction(functionID) {
		return s.invokePlatform(ctx, functionID, req)
	}

	// Regular function forwarding - need to dial game server
	addr, err := s.pickInstance(functionID, req.GetMetadata())
	if err != nil {
		return nil, err
	}

	// For now, return an error indicating we need to implement game server forwarding
	// This would require either:
	// 1. The game server to also speak NNG
	// 2. A gRPC→NNG bridge
	return nil, fmt.Errorf("game server forwarding not yet implemented for function %s at %s", functionID, addr)
}

// invokePlatform handles platform function calls
func (s *AgentServer) invokePlatform(ctx context.Context, functionID string, req *sdkv1.InvokeRequest) ([]byte, error) {
	request := req.GetPayload()

	response, err := s.platformManager.Call(ctx, functionID, request)
	if err != nil {
		return nil, fmt.Errorf("platform call failed: %w", err)
	}

	resp := &sdkv1.InvokeResponse{
		Payload: response,
	}
	return proto.Marshal(resp)
}

// pickInstance selects a game server instance for the function
func (s *AgentServer) pickInstance(functionID string, metadata map[string]string) (string, error) {
	if s.store == nil {
		return "", fmt.Errorf("instance store not initialized")
	}
	if functionID == "" {
		return "", fmt.Errorf("function ID is required")
	}

	snap := s.store.List()
	arr := snap[functionID]
	if len(arr) == 0 {
		return "", fmt.Errorf("function '%s' is not registered", functionID)
	}

	// Check if instances are healthy (not too old)
	now := time.Now()
	for _, inst := range arr {
		if now.Sub(inst.LastSeen) < 30*time.Second {
			return inst.Addr, nil
		}
	}

	// All instances are stale but return the first one
	return arr[0].Addr, fmt.Errorf("function '%s' instances are stale", functionID)
}

// handleStartJob handles StartJobRequest
func (s *AgentServer) handleStartJob(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest for StartJob: %w", err)
	}

	// For now, return not implemented
	// Full implementation would forward to game server and track job
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	resp := &sdkv1.StartJobResponse{
		JobId: jobID,
	}

	// Track the job
	if s.jobs != nil {
		s.jobs.Set(jobID, "")
	}

	return proto.Marshal(resp)
}

// handleCancelJob handles CancelJobRequest
func (s *AgentServer) handleCancelJob(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.CancelJobRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal CancelJobRequest: %w", err)
	}

	if s.jobs == nil {
		return nil, fmt.Errorf("job tracking not available")
	}

	if _, ok := s.jobs.Get(req.GetJobId()); ok {
		// Remove from tracking
		s.jobs.Delete(req.GetJobId())
		s.logger.Info("job cancelled", "job_id", req.GetJobId())
	}

	resp := &sdkv1.StartJobResponse{}
	return proto.Marshal(resp)
}

// handleGetSystemInfo handles GetSystemInfoRequest
func (s *AgentServer) handleGetSystemInfo(ctx context.Context, data []byte) ([]byte, error) {
	req := &emptypb.Empty{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal Empty: %w", err)
	}

	if s.opsServer == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := s.opsServer.GetSystemInfo(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get system info: %w", err)
	}

	return proto.Marshal(resp)
}

// handleListProcesses handles ListProcessesRequest
func (s *AgentServer) handleListProcesses(ctx context.Context, data []byte) ([]byte, error) {
	req := &emptypb.Empty{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal Empty: %w", err)
	}

	if s.opsServer == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := s.opsServer.ListProcesses(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	return proto.Marshal(resp)
}

// handleReportMetrics handles ReportMetricsRequest
func (s *AgentServer) handleReportMetrics(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.MetricsReport{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal MetricsReport: %w", err)
	}

	if s.opsServer == nil {
		// Just ack, don't error
		resp := &emptypb.Empty{}
		return proto.Marshal(resp)
	}

	resp, err := s.opsServer.ReportMetrics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("report metrics: %w", err)
	}

	return proto.Marshal(resp)
}

// handleRestartProcess handles RestartProcessRequest
func (s *AgentServer) handleRestartProcess(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.RestartProcessRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RestartProcessRequest: %w", err)
	}

	if s.opsServer == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := s.opsServer.RestartProcess(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("restart process: %w", err)
	}

	return proto.Marshal(resp)
}

// handleStopProcess handles StopProcessRequest
func (s *AgentServer) handleStopProcess(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.StopProcessRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal StopProcessRequest: %w", err)
	}

	if s.opsServer == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := s.opsServer.StopProcess(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("stop process: %w", err)
	}

	return proto.Marshal(resp)
}

// handleStartProcess handles StartProcessRequest
func (s *AgentServer) handleStartProcess(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.StartProcessRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal StartProcessRequest: %w", err)
	}

	if s.opsServer == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := s.opsServer.StartProcess(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	return proto.Marshal(resp)
}

// handleExecuteCommand handles ExecuteCommandRequest
func (s *AgentServer) handleExecuteCommand(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.ExecuteCommandRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ExecuteCommandRequest: %w", err)
	}

	if s.opsServer == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := s.opsServer.ExecuteCommand(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute command: %w", err)
	}

	return proto.Marshal(resp)
}

// handleRegisterLocal handles RegisterLocalRequest
func (s *AgentServer) handleRegisterLocal(ctx context.Context, data []byte) ([]byte, error) {
	req := &localv1.RegisterLocalRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterLocalRequest: %w", err)
	}

	if s.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	// Use the Register method which takes service_id, addr, version, and functions
	s.store.Register(req.ServiceId, req.RpcAddr, req.Version, req.Functions)

	resp := &localv1.RegisterLocalResponse{}
	return proto.Marshal(resp)
}

// handleHeartbeatLocal handles HeartbeatLocalRequest
func (s *AgentServer) handleHeartbeatLocal(ctx context.Context, data []byte) ([]byte, error) {
	req := &localv1.HeartbeatRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal HeartbeatRequest: %w", err)
	}

	if s.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	// Get current snapshot and update LastSeen for the service
	snap := s.store.List()
	for _, instances := range snap {
		for _, inst := range instances {
			if inst.ServiceID == req.ServiceId {
				// Update LastSeen by re-registering
				s.store.Register(inst.ServiceID, inst.Addr, inst.Version, nil)
			}
		}
	}

	resp := &localv1.HeartbeatResponse{}
	return proto.Marshal(resp)
}

// handleListLocal handles ListLocalRequest
func (s *AgentServer) handleListLocal(ctx context.Context, data []byte) ([]byte, error) {
	req := &localv1.ListLocalRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ListLocalRequest: %w", err)
	}

	if s.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	snap := s.store.List()
	functions := make([]*localv1.LocalFunction, 0, len(snap))

	for fid, instances := range snap {
		localInsts := make([]*localv1.LocalInstance, 0, len(instances))
		for _, inst := range instances {
			localInsts = append(localInsts, &localv1.LocalInstance{
				ServiceId: inst.ServiceID,
				Addr:      inst.Addr,
				Version:   inst.Version,
			})
		}

		functions = append(functions, &localv1.LocalFunction{
			Id:        fid,
			Instances: localInsts,
		})
	}

	resp := &localv1.ListLocalResponse{
		Functions: functions,
	}
	return proto.Marshal(resp)
}

// createErrorResponse creates an error response
func (s *AgentServer) createErrorResponse(err error) []byte {
	// Return a minimal response
	resp := &emptypb.Empty{}
	data, _ := proto.Marshal(resp)
	s.logger.Error("Request error", "error", err)
	return data
}

// jobIndex tracks running jobs
type jobIndex struct {
	mu   sync.RWMutex
	jobs map[string]string // job_id -> addr
}

func newJobIndex() *jobIndex {
	return &jobIndex{
		jobs: make(map[string]string),
	}
}

func (j *jobIndex) Set(jobID, addr string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.jobs[jobID] = addr
}

func (j *jobIndex) Get(jobID string) (string, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	addr, ok := j.jobs[jobID]
	return addr, ok
}

func (j *jobIndex) Delete(jobID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.jobs, jobID)
}

// hostFromAddr extracts hostname from address
func hostFromAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(strings.TrimPrefix(addr, "["), "]")
}

// fnvIndex computes hash-based index
func fnvIndex(key string, mod int) int {
	if mod <= 1 {
		return 0
	}
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return int(h % uint32(mod))
}
