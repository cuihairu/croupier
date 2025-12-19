// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
	"github.com/cuihairu/croupier/services/agent/internal/config"
)

type ServiceContext struct {
	Config        config.Config
	StartTime     time.Time
	Core          *agentcore.App
	LocalGRPCAddr string
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
