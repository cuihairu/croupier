package edge

import (
    ctrl "github.com/cuihairu/croupier/internal/platform/control"
    reg "github.com/cuihairu/croupier/internal/platform/registry"
    controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    jobv1 "github.com/cuihairu/croupier/pkg/pb/croupier/edge/job/v1"
    tunnelv1 "github.com/cuihairu/croupier/pkg/pb/croupier/tunnel/v1"
    "google.golang.org/grpc"
)

// App assembles gRPC services for Edge process. For now, tunnel/function/job are stubs.
type App struct {
    ctrl *ctrl.Server
}

func New(registry *reg.Store) *App {
    if registry == nil { registry = reg.NewStore() }
    return &App{ctrl: ctrl.NewServer(registry)}
}

// RegisterGRPC registers gRPC services on the given server.
func (a *App) RegisterGRPC(s *grpc.Server) {
    controlv1.RegisterControlServiceServer(s, a.ctrl)
    tunnelv1.RegisterTunnelServiceServer(s, &TunnelServer{})
    functionv1.RegisterFunctionServiceServer(s, &FunctionServer{})
    jobv1.RegisterJobServiceServer(s, &JobServer{})
}

// MetricsMap exposes aggregated metrics (placeholder for now).
func (a *App) MetricsMap() map[string]any { return map[string]any{} }

// TunnelServer is a stub implementation.
type TunnelServer struct{ tunnelv1.UnimplementedTunnelServiceServer }

// FunctionServer is a stub implementation.
type FunctionServer struct{ functionv1.UnimplementedFunctionServiceServer }

// JobServer is a stub implementation.
type JobServer struct{ jobv1.UnimplementedJobServiceServer }

