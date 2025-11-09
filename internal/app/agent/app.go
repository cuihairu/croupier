package agent

import (
    agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
    "google.golang.org/grpc"
)

// App assembles minimal gRPC services for Agent process.
type App struct {
    store *agentlocal.LocalStore
    jobs  *jobIndex
}

func New() *App { return &App{store: agentlocal.NewLocalStore(), jobs: newJobIndex()} }

func (a *App) RegisterGRPC(s *grpc.Server) {
    // Function service (local-forwarding implementation over protobuf)
    functionv1.RegisterFunctionServiceServer(s, &FunctionServer{store: a.store, jobs: a.jobs})
    // Local registration service provides RegisterLocal/Heartbeat/ListLocal
    localv1.RegisterLocalControlServiceServer(s, agentlocal.NewServer(a.store))
}

// FunctionServer implemented in function_server.go
