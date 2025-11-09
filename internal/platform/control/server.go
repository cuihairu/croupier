package control

import (
    "context"
    "time"

    reg "github.com/cuihairu/croupier/internal/platform/registry"
    controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
)

// Server implements the ControlService and exposes a registry store for other components.
type Server struct {
    controlv1.UnimplementedControlServiceServer
    reg *reg.Store
}

func NewServer(registry *reg.Store) *Server {
    if registry == nil {
        registry = reg.NewStore()
    }
    return &Server{reg: registry}
}

// Store returns the underlying registry Store (for function server / HTTP handlers).
func (s *Server) Store() *reg.Store { return s.reg }

// Register registers or updates an agent session. Minimal fields are accepted.
func (s *Server) Register(ctx context.Context, in *controlv1.RegisterRequest) (*controlv1.RegisterResponse, error) {
    if in == nil { return &controlv1.RegisterResponse{}, nil }
    sess := &reg.AgentSession{
        AgentID:  in.GetAgentId(),
        GameID:   in.GetGameId(),
        Env:      in.GetEnv(),
        RPCAddr:  in.GetRpcAddr(),
        Version:  in.GetVersion(),
        // Region/Zone/Labels are not present in current proto; leave empty
        ExpireAt: time.Now().Add(60 * time.Second),
        Functions: map[string]reg.FunctionMeta{},
    }
    // Populate functions from request descriptors (id -> enabled)
    if in.Functions != nil {
        for _, f := range in.Functions {
            if f == nil || f.GetId() == "" { continue }
            sess.Functions[f.GetId()] = reg.FunctionMeta{Enabled: f.GetEnabled()}
        }
    }
    s.reg.UpsertAgent(sess)
    return &controlv1.RegisterResponse{}, nil
}

// Heartbeat extends the expiry of an agent session.
func (s *Server) Heartbeat(ctx context.Context, in *controlv1.HeartbeatRequest) (*controlv1.HeartbeatResponse, error) {
    if in == nil || in.GetAgentId() == "" { return &controlv1.HeartbeatResponse{}, nil }
    s.reg.Mu().Lock()
    if a := s.reg.AgentsUnsafe()[in.GetAgentId()]; a != nil {
        a.ExpireAt = time.Now().Add(60 * time.Second)
    }
    s.reg.Mu().Unlock()
    return &controlv1.HeartbeatResponse{}, nil
}
