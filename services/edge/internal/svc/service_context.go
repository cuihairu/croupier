// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"sync"
	"time"

	edgeapp "github.com/cuihairu/croupier/internal/app/edge"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/edge/internal/config"
)

type ServiceContext struct {
	Config    config.Config
	EdgeApp   *edgeapp.App
	startTime time.Time
	State     *StateStore
}

func NewServiceContext(c config.Config) *ServiceContext {
	ctx := &ServiceContext{
		Config:    c,
		EdgeApp:   edgeapp.New(reg.NewStore()),
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
