package local

import (
    "context"
    "log/slog"
    "time"

    localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
    controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
    "github.com/cuihairu/croupier/internal/agent/registry"
    "github.com/cuihairu/croupier/internal/agent/jobs"
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
    exec *jobs.Executor
    // allowed functions (if empty -> allow all)
    allowed map[string]bool
}

func NewServer(store *registry.LocalStore, ctrl controlv1.ControlServiceClient, agentID, agentVersion, agentRPCAddr, gameID, env string, exec *jobs.Executor) *Server {
    return &Server{store: store, ctrl: ctrl, agentID: agentID, agentVersion: agentVersion, agentRPCAddr: agentRPCAddr, gameID: gameID, env: env, exec: exec, allowed: map[string]bool{}}
}

func (s *Server) RegisterLocal(ctx context.Context, req *localv1.RegisterLocalRequest) (*localv1.RegisterLocalResponse, error) {
    for _, f := range req.Functions { s.store.Add(f.Id, req.ServiceId, req.RpcAddr, f.Version) }
    slog.Info("local register", "service_id", req.ServiceId, "rpc_addr", req.RpcAddr, "functions", len(req.Functions))
    // Update Server with functions seen by Agent (DEV ONLY path)
    var fns []*controlv1.FunctionDescriptor
    for fid, arr := range s.store.List() {
        ver := ""
        if len(arr) > 0 { ver = arr[0].Version }
        fns = append(fns, &controlv1.FunctionDescriptor{Id: fid, Version: ver})
    }
    if s.ctrl != nil {
        if _, err := s.ctrl.Register(ctx, &controlv1.RegisterRequest{AgentId: s.agentID, Version: s.agentVersion, RpcAddr: s.agentRPCAddr, GameId: s.gameID, Env: s.env, Functions: fns}); err != nil {
            slog.Warn("core register update failed", "error", err.Error())
        }
    }
    return &localv1.RegisterLocalResponse{SessionId: "local-" + req.ServiceId}, nil
}

func (s *Server) Heartbeat(ctx context.Context, req *localv1.HeartbeatRequest) (*localv1.HeartbeatResponse, error) {
    // Best-effort touch all instances from this service
    s.store.TouchByService(req.ServiceId, "")
    return &localv1.HeartbeatResponse{}, nil
}

func (s *Server) ListLocal(ctx context.Context, _ *localv1.ListLocalRequest) (*localv1.ListLocalResponse, error) {
    out := &localv1.ListLocalResponse{}
    mp := s.store.List()
    for fid, arr := range mp {
        if len(s.allowed) > 0 { if !s.allowed[fid] { continue } }
        lf := &localv1.LocalFunction{Id: fid}
        for _, inst := range arr {
            lf.Instances = append(lf.Instances, &localv1.LocalInstance{ServiceId: inst.ServiceID, Addr: inst.Addr, Version: inst.Version, LastSeen: inst.LastSeen.Format(time.RFC3339)})
        }
        out.Functions = append(out.Functions, lf)
    }
    return out, nil
}

// UpdateAllowed sets the allow-list for functions (empty slice -> allow all).
func (s *Server) UpdateAllowed(fns []string) {
    m := map[string]bool{}
    for _, id := range fns { if id != "" { m[id] = true } }
    s.allowed = m
}

func (s *Server) GetJobResult(ctx context.Context, req *localv1.GetJobResultRequest) (*localv1.GetJobResultResponse, error) {
    if s.exec == nil { return &localv1.GetJobResultResponse{State: "unknown"}, nil }
    st, ok := s.exec.Status(req.JobId)
    if !ok { return &localv1.GetJobResultResponse{State: "unknown"}, nil }
    return &localv1.GetJobResultResponse{State: st.State, Payload: st.Payload, Error: st.Error}, nil
}
