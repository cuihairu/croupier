package agent

import (
	"context"
	"errors"
	"os"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/grpc"
)

// App assembles minimal gRPC services for Agent process.
type App struct {
	store    *agentlocal.LocalStore
	jobs     *jobIndex
	upstream *UpstreamClient
	outTLS   *tlsutil.ClientTLSConfig
}

func New(serverAddr, agentID string) *App {
	store := agentlocal.NewLocalStore()
	return &App{
		store:    store,
		jobs:     newJobIndex(),
		upstream: NewUpstreamClient(serverAddr, agentID, store, nil),
	}
}

func (a *App) RegisterGRPC(s *grpc.Server) {
	// Function service (local-forwarding implementation over protobuf)
	functionv1.RegisterFunctionServiceServer(s, &FunctionServer{store: a.store, jobs: a.jobs, tlsCfg: a.outTLS})
	// Local registration service provides RegisterLocal/Heartbeat/ListLocal
	localv1.RegisterLocalControlServiceServer(s, agentlocal.NewServer(a.store))
}

// Run starts the agent's background processes (upstream sync).
func (a *App) Run(ctx context.Context) error {
	a.startMaintenance(ctx)
	return a.upstream.Start(ctx)
}

func (a *App) startMaintenance(ctx context.Context) {
	if a == nil || a.store == nil {
		return
	}

	pruneInterval := parseDurationEnv("CROUPIER_AGENTLOCAL_PRUNE_INTERVAL", 30*time.Second)
	maxAge := parseDurationEnv("CROUPIER_AGENTLOCAL_MAX_AGE", 2*time.Minute)
	jobResultMaxAge := parseDurationEnv("CROUPIER_AGENTLOCAL_JOBRESULT_MAX_AGE", 10*time.Minute)
	if pruneInterval <= 0 || maxAge <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(pruneInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.store.Prune(maxAge)
				if jobResultMaxAge > 0 {
					a.store.CleanupOldJobResults(jobResultMaxAge)
				}
			}
		}
	}()
}

func parseDurationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// Stop shuts down background upstream connection.
func (a *App) Stop() {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.Stop()
}

// WithUpstreamMetadata updates metadata fields propagated to the control server.
func (a *App) WithUpstreamMetadata(meta UpstreamMetadata) {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.WithMetadata(meta)
}

func (a *App) WithUpstreamTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.SetTLSConfig(cfg)
}

func (a *App) WithOutboundTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	if a == nil {
		return
	}
	a.outTLS = cfg
}

// Store exposes the local instance registry.
func (a *App) Store() *agentlocal.LocalStore {
	if a == nil {
		return nil
	}
	return a.store
}

// SyncUpstream forces a best-effort upstream register call.
func (a *App) SyncUpstream(ctx context.Context) error {
	if a == nil || a.upstream == nil {
		return errors.New("agent upstream not initialized")
	}
	return a.upstream.Sync(ctx)
}

// HeartbeatUpstream triggers a best-effort upstream heartbeat call.
func (a *App) HeartbeatUpstream(ctx context.Context) error {
	if a == nil || a.upstream == nil {
		return errors.New("agent upstream not initialized")
	}
	return a.upstream.Heartbeat(ctx)
}

// FunctionServer implemented in function_server.go
