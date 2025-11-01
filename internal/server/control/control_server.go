package control

import (
    "context"
    "log/slog"
    "time"

    controlv1 "github.com/cuihairu/croupier/gen/go/croupier/control/v1"
    "github.com/cuihairu/croupier/internal/server/registry"
    "github.com/cuihairu/croupier/internal/server/games"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// Server implements the ControlService.
type Server struct {
    controlv1.UnimplementedControlServiceServer
    store *registry.Store
    games *games.Store
}

func NewServer(g *games.Store) *Server { return &Server{store: registry.NewStore(), games: g} }

// Store exposes the registry for other servers (FunctionService) to route requests.
func (s *Server) Store() *registry.Store { return s.store }

func (s *Server) Register(ctx context.Context, req *controlv1.RegisterRequest) (*controlv1.RegisterResponse, error) {
    // TODO: validate req, upsert agent session, index functions
    slog.Info("control register", "agent_id", req.GetAgentId(), "version", req.GetVersion(), "game_id", req.GetGameId(), "env", req.GetEnv(), "functions", len(req.GetFunctions()))
    // Gate by allowed games
    if s.games != nil {
        if req.GetGameId() == "" || !s.games.IsAllowed(req.GetGameId(), req.GetEnv()) {
            return nil, status.Error(codes.PermissionDenied, "game not allowed; ask admin to add game_id first")
        }
    }
    fset := map[string]bool{}
    for _, f := range req.GetFunctions() { fset[f.GetId()] = true }
    s.store.UpsertAgent(&registry.AgentSession{
        AgentID: req.GetAgentId(),
        Version: req.GetVersion(),
        RPCAddr: req.GetRpcAddr(),
        GameID:  req.GetGameId(),
        Env:     req.GetEnv(),
        Functions: fset,
        ExpireAt: time.Now().Add(24 * time.Hour),
    })
    return &controlv1.RegisterResponse{SessionId: "sess-" + req.GetAgentId(), ExpireAt: time.Now().Add(24 * time.Hour).Unix()}, nil
}

func (s *Server) Heartbeat(ctx context.Context, req *controlv1.HeartbeatRequest) (*controlv1.HeartbeatResponse, error) {
    // TODO: refresh session lease, track liveness
    return &controlv1.HeartbeatResponse{}, nil
}
