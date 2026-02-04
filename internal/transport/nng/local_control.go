// Package nng provides NNG (Nanomsg Next Generation) transport layer for Croupier.
package nng

import (
	"context"
	"fmt"
	"sync"
	"time"

	localv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/agent/local/v1"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/transport/nng/protocol"
)

// LocalControlServer handles NNG-based SDK registration and function invocation.
type LocalControlServer struct {
	store           *agentlocal.LocalStore
	functionHandler FunctionInvoker

	mu       sync.RWMutex
	sessions map[string]*SessionInfo
}

// SessionInfo holds session information for a registered SDK.
type SessionInfo struct {
	SessionID     string
	ServiceID     string
	Version       string
	RPCAddr       string
	Functions     []*localv1.LocalFunctionDescriptor
	LastHeartbeat time.Time
}

// FunctionInvoker is the interface for invoking functions on registered SDKs.
type FunctionInvoker interface {
	Invoke(ctx context.Context, serviceID, functionID string, payload []byte) ([]byte, error)
}

// NewLocalControlServer creates a new NNG-based local control server.
func NewLocalControlServer(store *agentlocal.LocalStore, functionHandler FunctionInvoker) *LocalControlServer {
	return &LocalControlServer{
		store:           store,
		functionHandler: functionHandler,
		sessions:        make(map[string]*SessionInfo),
	}
}

// Handle implements the Handler interface for NNG requests.
func (s *LocalControlServer) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgRegisterClientRequest:
		return s.handleRegister(ctx, reqID, body)
	case protocol.MsgClientHeartbeatRequest:
		return s.handleHeartbeat(ctx, reqID, body)
	case protocol.MsgInvokeRequest:
		return s.handleInvoke(ctx, reqID, body)
	case protocol.MsgStartJobRequest:
		return s.handleStartJob(ctx, reqID, body)
	case protocol.MsgCancelJobRequest:
		return s.handleCancelJob(ctx, reqID, body)
	default:
		return nil, fmt.Errorf("unknown message type: %s", protocol.MsgIDString(msgID))
	}
}

// handleRegister handles SDK registration requests.
func (s *LocalControlServer) handleRegister(ctx context.Context, reqID uint32, body []byte) ([]byte, error) {
	req := &localv1.RegisterLocalRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}

	// Generate session ID
	sessionID := generateSessionID(req.ServiceId)

	// Store session info
	s.mu.Lock()
	s.sessions[sessionID] = &SessionInfo{
		SessionID:     sessionID,
		ServiceID:     req.ServiceId,
		Version:       req.Version,
		RPCAddr:       req.RpcAddr,
		Functions:     req.Functions,
		LastHeartbeat: time.Now(),
	}
	s.mu.Unlock()

	// Store in agentlocal store
	s.store.Register(req.ServiceId, req.RpcAddr, req.Version, req.Functions)

	// Build response
	resp := &localv1.RegisterLocalResponse{
		SessionId: sessionID,
	}
	return proto.Marshal(resp)
}

// handleHeartbeat handles heartbeat requests.
func (s *LocalControlServer) handleHeartbeat(ctx context.Context, reqID uint32, body []byte) ([]byte, error) {
	req := &localv1.HeartbeatRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}

	s.mu.Lock()
	if session, ok := s.sessions[req.SessionId]; ok {
		session.LastHeartbeat = time.Now()
	}
	s.mu.Unlock()

	resp := &localv1.HeartbeatResponse{}
	return proto.Marshal(resp)
}

// handleInvoke handles function invocation requests.
func (s *LocalControlServer) handleInvoke(ctx context.Context, reqID uint32, body []byte) ([]byte, error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}

	// Find the session and service for this function
	var serviceID string
	s.mu.RLock()
	for _, session := range s.sessions {
		for _, fn := range session.Functions {
			if fn.Id == req.FunctionId {
				serviceID = session.ServiceID
				break
			}
		}
		if serviceID != "" {
			break
		}
	}
	s.mu.RUnlock()

	if serviceID == "" {
		return nil, fmt.Errorf("function not found: %s", req.FunctionId)
	}

	// Invoke the function
	if s.functionHandler == nil {
		return nil, fmt.Errorf("function handler not configured")
	}

	result, err := s.functionHandler.Invoke(ctx, serviceID, req.FunctionId, req.Payload)
	if err != nil {
		return nil, fmt.Errorf("invoke failed: %w", err)
	}

	resp := &sdkv1.InvokeResponse{
		Payload: result,
	}
	return proto.Marshal(resp)
}

// handleStartJob handles async job start requests.
func (s *LocalControlServer) handleStartJob(ctx context.Context, reqID uint32, body []byte) ([]byte, error) {
	// TODO: Implement async job support
	return nil, fmt.Errorf("StartJob not yet implemented")
}

// handleCancelJob handles job cancellation requests.
func (s *LocalControlServer) handleCancelJob(ctx context.Context, reqID uint32, body []byte) ([]byte, error) {
	// TODO: Implement job cancellation
	return nil, fmt.Errorf("CancelJob not yet implemented")
}

// GetSession returns session info by session ID.
func (s *LocalControlServer) GetSession(sessionID string) (*SessionInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.sessions[sessionID]
	return info, ok
}

// CleanupInactiveSessions removes sessions that haven't sent heartbeat in a while.
func (s *LocalControlServer) CleanupInactiveSessions(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for sessionID, session := range s.sessions {
		if now.Sub(session.LastHeartbeat) > timeout {
			delete(s.sessions, sessionID)
			// Note: The agentlocal store doesn't have a Deregister method,
			// inactive instances will be naturally pruned by the store's Prune method
		}
	}
}

// generateSessionID generates a unique session ID.
func generateSessionID(serviceID string) string {
	return fmt.Sprintf("%s-%d", serviceID, time.Now().UnixNano())
}
