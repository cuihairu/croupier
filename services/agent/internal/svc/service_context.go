// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"sync"
	"time"

	"github.com/cuihairu/croupier/services/agent/internal/config"
)

type ServiceContext struct {
	Config     config.Config
	AgentState *StateStore
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:     c,
		AgentState: newStateStore(),
	}
}

type StateStore struct {
	StartTime time.Time
	Mu        sync.RWMutex
	Agents    map[string]*AgentRecord
	Functions map[string]*FunctionRecord
	Jobs      map[string]*JobRecord
}

type AgentRecord struct {
	ID            string
	GameID        string
	Env           string
	Type          string
	Version       string
	RPCAddr       string
	Status        string
	Functions     int64
	Metadata      map[string]string
	RegisteredAt  time.Time
	LastHeartbeat time.Time
}

type FunctionRecord struct {
	ID         string
	GameID     string
	Env        string
	Descriptor map[string]interface{}
	Schema     map[string]interface{}
	Metadata   map[string]interface{}
	Registered time.Time
}

type JobRecord struct {
	ID         string
	FunctionID string
	GameID     string
	Env        string
	Status     string
	Result     map[string]interface{}
	Error      string
	StartTime  time.Time
	EndTime    *time.Time
}

func newStateStore() *StateStore {
	return &StateStore{
		StartTime: time.Now(),
		Agents:    make(map[string]*AgentRecord),
		Functions: make(map[string]*FunctionRecord),
		Jobs:      make(map[string]*JobRecord),
	}
}

func (s *StateStore) Uptime() time.Duration {
	return time.Since(s.StartTime)
}

func CloneStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func CloneInterfaceMap(m map[string]interface{}) map[string]interface{} {
	if len(m) == 0 {
		return nil
	}
	cp := make(map[string]interface{}, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
