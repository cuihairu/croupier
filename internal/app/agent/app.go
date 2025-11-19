package agent

import (
	"context"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/grpc"
)

// App assembles minimal gRPC services for Agent process.
type App struct {
	store    *agentlocal.LocalStore
	jobs     *jobIndex
	upstream *UpstreamClient
}

func New(serverAddr, agentID string) *App {
	store := agentlocal.NewLocalStore()
	return &App{
		store:    store,
		jobs:     newJobIndex(),
		upstream: NewUpstreamClient(serverAddr, agentID, store),
	}
}

func (a *App) RegisterGRPC(s *grpc.Server) {
	// Function service (local-forwarding implementation over protobuf)
	functionv1.RegisterFunctionServiceServer(s, &FunctionServer{store: a.store, jobs: a.jobs})
	// Local registration service provides RegisterLocal/Heartbeat/ListLocal
	localv1.RegisterLocalControlServiceServer(s, agentlocal.NewServer(a.store))
}

// Run starts the agent's background processes (upstream sync).
func (a *App) Run(ctx context.Context) error {
	return a.upstream.Start(ctx)
}

// FunctionServer implemented in function_server.go
