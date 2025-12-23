// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"strings"
	"sync"
	"time"

	edgeapp "github.com/cuihairu/croupier/internal/app/edge"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/edge/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config    config.Config
	EdgeApp   *edgeapp.App
	startTime time.Time
	State     *StateStore
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
