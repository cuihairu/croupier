// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"sync"
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
	"github.com/cuihairu/croupier/services/agent/internal/config"
)

type ServiceContext struct {
	Config        config.Config
	StartTime     time.Time
	Core          *agentcore.App
	LocalGRPCAddr string

	// Registration and heartbeat timestamps
	mu            sync.RWMutex
	RegisteredAt  time.Time
	LastHeartbeat time.Time
}

func NewServiceContext(c config.Config, core *agentcore.App, localGRPCAddr string) *ServiceContext {
	return &ServiceContext{
		Config:        c,
		StartTime:     time.Now(),
		Core:          core,
		LocalGRPCAddr: localGRPCAddr,
	}
}

func (s *ServiceContext) Uptime() time.Duration {
	if s == nil || s.StartTime.IsZero() {
		return 0
	}
	return time.Since(s.StartTime)
}

// SetRegisteredAt sets the registration timestamp.
func (s *ServiceContext) SetRegisteredAt(t time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RegisteredAt = t
}

// GetRegisteredAt returns the registration timestamp.
func (s *ServiceContext) GetRegisteredAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.RegisteredAt
}

// SetLastHeartbeat sets the last heartbeat timestamp.
func (s *ServiceContext) SetLastHeartbeat(t time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastHeartbeat = t
}

// GetLastHeartbeat returns the last heartbeat timestamp.
func (s *ServiceContext) GetLastHeartbeat() time.Time {
	if s == nil {
		return time.Time{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastHeartbeat
}
