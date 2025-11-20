package svc

import (
	"context"
	"sync"

	"github.com/cuihairu/croupier/services/agent/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config     config.Config
	AgentStore *AgentStore
	JobManager *JobManager
}

type AgentStore struct {
	mu          sync.RWMutex
	agentInfo   map[string]interface{}
	functions   map[string]interface{}
	jobs        map[string]interface{}
}

type JobManager struct {
	mu       sync.RWMutex
	jobs     map[string]*Job
	maxJobs  int
	running  int
}

type Job struct {
	ID            string                 `json:"id"`
	FunctionID    string                 `json:"function_id"`
	GameID        string                 `json:"game_id"`
	Env           string                 `json:"env"`
	Inputs        map[string]interface{} `json:"inputs"`
	Options       map[string]interface{} `json:"options"`
	Status        string                 `json:"status"`
	Result        map[string]interface{} `json:"result"`
	Error         string                 `json:"error"`
	Progress      int64                  `json:"progress"`
	StartTime     string                 `json:"start_time"`
	EndTime       string                 `json:"end_time"`
	Retries       int                    `json:"retries"`
	MaxRetries    int                    `json:"max_retries"`
	FunctionInfo  interface{}            `json:"function_info"`
	ctx           context.Context
	cancelFunc    context.CancelFunc
}

func NewServiceContext(c config.Config) *ServiceContext {
	logx.Info("Initializing agent service context")

	return &ServiceContext{
		Config:     c,
		AgentStore: NewAgentStore(),
		JobManager: NewJobManager(c.Job.MaxConcurrent),
	}
}

func NewAgentStore() *AgentStore {
	return &AgentStore{
		agentInfo: make(map[string]interface{}),
		functions: make(map[string]interface{}),
		jobs:      make(map[string]interface{}),
	}
}

func NewJobManager(maxJobs int) *JobManager {
	return &JobManager{
		jobs:     make(map[string]*Job),
		maxJobs:  maxJobs,
		running:  0,
	}
}

func (s *AgentStore) SetAgentInfo(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentInfo[key] = value
}

func (s *AgentStore) GetAgentInfo(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.agentInfo[key]
	return val, ok
}

func (s *AgentStore) RegisterFunction(id string, descriptor interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.functions[id] = descriptor
}

func (s *AgentStore) GetFunction(id string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.functions[id]
	return val, ok
}

func (s *AgentStore) ListFunctions() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range s.functions {
		result[k] = v
	}
	return result
}

func (jm *JobManager) CreateJob(job *Job) bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if jm.running >= jm.maxJobs {
		return false
	}

	job.ctx, job.cancelFunc = context.WithCancel(context.Background())
	jm.jobs[job.ID] = job
	jm.running++
	return true
}

func (jm *JobManager) GetJob(id string) (*Job, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	job, ok := jm.jobs[id]
	return job, ok
}

func (jm *JobManager) UpdateJobStatus(id, status string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if job, ok := jm.jobs[id]; ok {
		job.Status = status
		if status == "completed" || status == "failed" {
			jm.running--
		}
	}
}

func (jm *JobManager) ListJobs() map[string]*Job {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	result := make(map[string]*Job)
	for k, v := range jm.jobs {
		result[k] = v
	}
	return result
}

func (jm *JobManager) CancelJob(id string) bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if job, ok := jm.jobs[id]; ok {
		job.cancelFunc()
		job.Status = "cancelled"
		jm.running--
		return true
	}
	return false
}