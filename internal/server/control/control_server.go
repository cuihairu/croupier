package control

import (
    "context"
    "log"
    "time"

    controlv1 "github.com/your-org/croupier/gen/go/croupier/control/v1"
    "github.com/your-org/croupier/internal/server/registry"
)

// Server implements the ControlService.
type Server struct {
    controlv1.UnimplementedControlServiceServer
    store *registry.Store
}

func NewServer() *Server { return &Server{store: registry.NewStore()} }

// Store exposes the registry for other servers (FunctionService) to route requests.
func (s *Server) Store() *registry.Store { return s.store }

func (s *Server) Register(ctx context.Context, req *controlv1.RegisterRequest) (*controlv1.RegisterResponse, error) {
    // TODO: validate req, upsert agent session, index functions
    log.Printf("control:Register agent_id=%s version=%s functions=%d", req.GetAgentId(), req.GetVersion(), len(req.GetFunctions()))
    fset := map[string]bool{}
    for _, f := range req.GetFunctions() { fset[f.GetId()] = true }
    s.store.UpsertAgent(&registry.AgentSession{
        AgentID: req.GetAgentId(),
        Version: req.GetVersion(),
        RPCAddr: req.GetRpcAddr(),
        Functions: fset,
        ExpireAt: time.Now().Add(24 * time.Hour),
    })
    return &controlv1.RegisterResponse{SessionId: "sess-" + req.GetAgentId(), ExpireAt: time.Now().Add(24 * time.Hour).Unix()}, nil
}

func (s *Server) Heartbeat(ctx context.Context, req *controlv1.HeartbeatRequest) (*controlv1.HeartbeatResponse, error) {
    // TODO: refresh session lease, track liveness
    return &controlv1.HeartbeatResponse{}, nil
}
