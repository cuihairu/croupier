package local

import (
    "context"
    "log"

    localv1 "github.com/your-org/croupier/gen/go/croupier/agent/local/v1"
    controlv1 "github.com/your-org/croupier/gen/go/croupier/control/v1"
    "github.com/your-org/croupier/internal/agent/registry"
)

type Server struct {
    localv1.UnimplementedLocalControlServiceServer
    store *registry.LocalStore
    ctrl  controlv1.ControlServiceClient
    agentID string
    agentVersion string
    agentRPCAddr string
    gameID string
    env    string
}

func NewServer(store *registry.LocalStore, ctrl controlv1.ControlServiceClient, agentID, agentVersion, agentRPCAddr, gameID, env string) *Server {
    return &Server{store: store, ctrl: ctrl, agentID: agentID, agentVersion: agentVersion, agentRPCAddr: agentRPCAddr, gameID: gameID, env: env}
}

func (s *Server) RegisterLocal(ctx context.Context, req *localv1.RegisterLocalRequest) (*localv1.RegisterLocalResponse, error) {
    for _, f := range req.Functions { s.store.Set(f.Id, req.RpcAddr, f.Version) }
    log.Printf("local register: service=%s addr=%s functions=%d", req.ServiceId, req.RpcAddr, len(req.Functions))
    // Update Core with functions seen by Agent (DEV ONLY path)
    var fns []*controlv1.FunctionDescriptor
    for fid, e := range s.store.List() {
        fns = append(fns, &controlv1.FunctionDescriptor{Id: fid, Version: e.Version})
    }
    if s.ctrl != nil {
        if _, err := s.ctrl.Register(ctx, &controlv1.RegisterRequest{AgentId: s.agentID, Version: s.agentVersion, RpcAddr: s.agentRPCAddr, GameId: s.gameID, Env: s.env, Functions: fns}); err != nil {
            log.Printf("core register update failed: %v", err)
        }
    }
    return &localv1.RegisterLocalResponse{SessionId: "local-" + req.ServiceId}, nil
}

func (s *Server) Heartbeat(ctx context.Context, req *localv1.HeartbeatRequest) (*localv1.HeartbeatResponse, error) {
    return &localv1.HeartbeatResponse{}, nil
}
