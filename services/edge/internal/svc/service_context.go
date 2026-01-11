// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"context"
	"strings"
	"sync"
	"time"

	serverv1 "github.com/cuihairu/croupier/generated/croupier/server/v1"
	edgeapp "github.com/cuihairu/croupier/internal/app/edge"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/services/edge/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ServiceContext struct {
	Config    config.Config
	EdgeApp   *edgeapp.App
	startTime time.Time
	State     *StateStore
	Upstream  *grpc.ClientConn
}

func NewServiceContext(c config.Config) *ServiceContext {
	registry := reg.NewStore()

	var jobStore dispatch.JobRoutingStore
	jobRoutingDir := strings.TrimSpace(c.Dispatch.JobRoutingDir)
	if jobRoutingDir != "" {
		store, err := dispatch.NewFileJobRoutingStore(jobRoutingDir)
		if err != nil {
			logx.Errorf("failed to init job routing store (dir=%s): %v", jobRoutingDir, err)
		} else {
			jobStore = store
		}
	}

	app := edgeapp.NewWithJobStore(registry, jobStore)

	if app != nil && app.Dispatcher() != nil && c.Dispatch.AgentTLS.Enabled {
		app.Dispatcher().SetTLSConfig(&tlsutil.ClientTLSConfig{
			CertFile:           strings.TrimSpace(c.Dispatch.AgentTLS.CertFile),
			KeyFile:            strings.TrimSpace(c.Dispatch.AgentTLS.KeyFile),
			CAFile:             strings.TrimSpace(c.Dispatch.AgentTLS.CAFile),
			ServerName:         strings.TrimSpace(c.Dispatch.AgentTLS.ServerName),
			InsecureSkipVerify: c.Dispatch.AgentTLS.InsecureSkipVerify,
		})
	}

	var upstreamConn *grpc.ClientConn
	if addr := strings.TrimSpace(c.Upstream.Addr); addr != "" {
		conn, err := dialUpstreamControl(c)
		if err != nil {
			logx.Errorf("failed to dial upstream control %q: %v", addr, err)
		} else {
			upstreamConn = conn
			app.SetUpstreamControlClient(serverv1.NewControlServiceClient(conn))
			logx.Infof("edge control proxy enabled (upstream=%s)", addr)
		}
	}
	if ttlStr := strings.TrimSpace(c.Dispatch.JobRoutingTTL); ttlStr != "" {
		if ttl, err := time.ParseDuration(ttlStr); err != nil {
			logx.Errorf("invalid dispatch.job_routing_ttl=%q: %v", ttlStr, err)
		} else if ttl > 0 {
			if err := app.CleanupOldJobs(ttl); err != nil {
				logx.Errorf("failed to cleanup old jobs: %v", err)
			}
		}
	}

	ctx := &ServiceContext{
		Config:    c,
		EdgeApp:   app,
		startTime: time.Now(),
		State:     newStateStore(),
		Upstream:  upstreamConn,
	}
	return ctx
}

func (s *ServiceContext) Uptime() time.Duration {
	if s == nil {
		return 0
	}
	return time.Since(s.startTime)
}

type StateStore struct {
	Mu      sync.RWMutex
	start   time.Time
	Tunnels map[string]*TunnelRecord
}

type TunnelRecord struct {
	ID          string
	AgentID     string
	ServerID    string
	Protocol    string
	RemoteAddr  string
	LocalAddr   string
	Status      string
	Connections int64
	BytesIn     int64
	BytesOut    int64
	Options     map[string]interface{}
	CreatedAt   time.Time
	LastActive  time.Time
}

func newStateStore() *StateStore {
	return &StateStore{
		start:   time.Now(),
		Tunnels: make(map[string]*TunnelRecord),
	}
}

func (s *StateStore) uptimeSeconds() int64 {
	if s == nil {
		return 0
	}
	return int64(time.Since(s.start).Seconds())
}

func CloneMap(m map[string]interface{}) map[string]interface{} {
	if len(m) == 0 {
		return nil
	}
	cp := make(map[string]interface{}, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func dialUpstreamControl(c config.Config) (*grpc.ClientConn, error) {
	addr := strings.TrimSpace(c.Upstream.Addr)
	if addr == "" {
		return nil, nil
	}

	var dialOpt grpc.DialOption
	if c.Upstream.Insecure {
		dialOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
	} else {
		creds, err := tlsutil.ClientTLSFromConfig(tlsutil.ClientTLSConfig{
			CertFile:           strings.TrimSpace(c.Upstream.TLSCertFile),
			KeyFile:            strings.TrimSpace(c.Upstream.TLSKeyFile),
			CAFile:             strings.TrimSpace(c.Upstream.CAFile),
			ServerName:         strings.TrimSpace(c.Upstream.ServerName),
			InsecureSkipVerify: c.Upstream.InsecureSkipVerify,
		})
		if err != nil {
			return nil, err
		}
		dialOpt = grpc.WithTransportCredentials(creds)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return grpc.DialContext(ctx, addr, dialOpt, grpc.WithBlock())
}
