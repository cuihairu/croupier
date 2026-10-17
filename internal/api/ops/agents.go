package ops

import (
	"context"
	"errors"
	"fmt"

	"github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// Agent operations sub-service

type AgentService struct {
	svcCtx *svc.ServiceContext
}

func NewAgentService(svcCtx *svc.ServiceContext) *AgentService {
	return &AgentService{svcCtx: svcCtx}
}

func (s *AgentService) List(ctx context.Context, gameId, env, status string) ([]OpsAgentInfo, error) {
	store := s.svcCtx.RegistryStore
	if store == nil {
		return []OpsAgentInfo{}, nil
	}

	store.Mu().RLock()
	local := make(map[string]*registry.AgentSession, 0)
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil {
			local[sess.AgentID] = sess
		}
	}
	store.Mu().RUnlock()

	agents := make([]OpsAgentInfo, 0)

	// 集群模式：以共享归属表为全集（本地 registry 只含连到本实例的
	// agent，若只读本地，入口分流到不同实例时列表会随机变化）。
	// 本地持有的补全详情（函数列表/版本/标签），非本地的标注 owner。
	if s.svcCtx.Cluster != nil && s.svcCtx.Cluster.ListAgentOwners != nil {
		owners, err := s.svcCtx.Cluster.ListAgentOwners(ctx)
		if err == nil {
			for _, rec := range owners {
				if gameId != "" && rec.GameID != gameId {
					continue
				}
				if env != "" && rec.Env != env {
					continue
				}
				info := OpsAgentInfo{
					AgentID:  rec.AgentID,
					GameID:   rec.GameID,
					Env:      rec.Env,
					LastSeen: utils.FormatTimestamp(rec.LastSeenAt),
				}
				if sess, ok := local[rec.AgentID]; ok {
					info.Connected = true
					info.Addr = sess.Addr
					info.Version = sess.Version
					info.Labels = sess.Labels
					functions := make([]string, 0, len(sess.Functions))
					for fid := range sess.Functions {
						functions = append(functions, fid)
					}
					info.Functions = functions
				} else {
					info.OwnerInstance = rec.InstanceID
				}
				agents = append(agents, info)
			}
			return agents, nil
		}
		// 归属表读取失败时退回本地视图（诊断意义大于精确）
	}

	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if gameId != "" && sess.GameID != gameId {
			continue
		}
		if env != "" && sess.Env != env {
			continue
		}

		functions := make([]string, 0, len(sess.Functions))
		for fid := range sess.Functions {
			functions = append(functions, fid)
		}

		agents = append(agents, OpsAgentInfo{
			AgentID:   sess.AgentID,
			Addr:      sess.Addr,
			GameID:    sess.GameID,
			Env:       sess.Env,
			Version:   sess.Version,
			Connected: true,
			LastSeen:  utils.FormatTimestamp(sess.LastSeen),
			Labels:    sess.Labels,
			Functions: functions,
		})
	}

	return agents, nil
}

func (s *AgentService) GetMeta(ctx context.Context, agentId string) (*OpsAgentSystemInfo, error) {
	store := s.svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == agentId {
			return &OpsAgentSystemInfo{
				OS:       sess.Labels["os"],
				Arch:     sess.Labels["arch"],
				Hostname: sess.Labels["hostname"],
			}, nil
		}
	}

	return nil, errors.New("agent not found")
}

func (s *AgentService) requireAgentScope(ctx context.Context, agentId string) error {
	if s.svcCtx == nil || s.svcCtx.RegistryStore == nil {
		return errors.New("registry store unavailable")
	}
	store := s.svcCtx.RegistryStore
	scope := svc.GameScopeFromContext(ctx)

	var clusterScope *registry.AgentSession
	if s.svcCtx.Cluster != nil && s.svcCtx.Cluster.Resolver != nil {
		if owner, err := s.svcCtx.Cluster.Resolver.FindOwner(ctx, agentId); err == nil && owner != nil {
			clusterScope = &registry.AgentSession{GameID: owner.GameID, Env: owner.Env}
		}
	}

	var session *registry.AgentSession
	store.Mu().RLock()
	session = store.PreviousAgentSession(agentId)
	store.Mu().RUnlock()
	if session == nil {
		if clusterScope != nil {
			session = clusterScope
		} else {
			return fmt.Errorf("agent %q not found", agentId)
		}
	}
	if scope.GameID != "" && session.GameID != scope.GameID {
		return fmt.Errorf("agent %q does not belong to game %q", agentId, scope.GameID)
	}
	if scope.Env != "" && session.Env != scope.Env {
		return fmt.Errorf("agent %q does not belong to env %q", agentId, scope.Env)
	}
	return nil
}

// GetSystemInfo fetches live system info from the agent over the ops tunnel
// (§10.5 fingerprint rides on SystemInfo).
func (s *AgentService) GetSystemInfo(ctx context.Context, agentId string) (*opsv1.SystemInfo, error) {
	if err := s.requireAgentScope(ctx, agentId); err != nil {
		return nil, err
	}
	client := ops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, agentId)
	if err != nil {
		return nil, errors.New("ops client unavailable: " + err.Error())
	}

	info, err := wrapper.GetSystemInfo(ctx)
	if err != nil {
		return nil, errors.New("get system info: " + err.Error())
	}
	return info, nil
}

func (s *AgentService) ExecCommand(ctx context.Context, agentId, command string, args []string, timeout int) (*OpsExecCommandResult, error) {
	if err := s.requireAgentScope(ctx, agentId); err != nil {
		return nil, err
	}
	client := ops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, agentId)
	if err != nil {
		return nil, errors.New("ops client unavailable: " + err.Error())
	}

	// Build ExecuteCommandRequest
	req := &opsv1.ExecuteCommandRequest{
		Command:        command,
		Args:           args,
		TimeoutSeconds: int32(timeout),
	}

	result, err := wrapper.ExecuteCommand(ctx, req)
	if err != nil {
		return nil, err
	}

	return &OpsExecCommandResult{
		ExitCode: result.ExitCode,
		Stdout:   result.StdOut,
		Stderr:   result.StdErr,
	}, nil
}

func (s *AgentService) StartProcess(ctx context.Context, agentId, command string, args, env []string, dir string) (int, error) {
	if err := s.requireAgentScope(ctx, agentId); err != nil {
		return 0, err
	}
	client := ops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, agentId)
	if err != nil {
		return 0, errors.New("ops client unavailable: " + err.Error())
	}

	req := &opsv1.StartProcessRequest{
		ProcessName: command,
	}

	resp, err := wrapper.StartProcess(ctx, req)
	if err != nil {
		return 0, err
	}

	return int(resp.Pid), nil
}

func (s *AgentService) StopProcess(ctx context.Context, agentId, processName string) error {
	if err := s.requireAgentScope(ctx, agentId); err != nil {
		return err
	}
	client := ops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, agentId)
	if err != nil {
		return errors.New("ops client unavailable: " + err.Error())
	}

	req := &opsv1.StopProcessRequest{
		ProcessName: processName,
	}

	_, err = wrapper.StopProcess(ctx, req)
	return err
}

// UpdateProcess swaps a managed process binary on the agent (§10.4 更新状态+指纹).
func (s *AgentService) UpdateProcess(ctx context.Context, agentId, processName, stagedPath, expectedSha256 string) (*opsv1.UpdateProcessResponse, error) {
	if err := s.requireAgentScope(ctx, agentId); err != nil {
		return nil, err
	}
	client := ops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, agentId)
	if err != nil {
		return nil, errors.New("ops client unavailable: " + err.Error())
	}

	req := &opsv1.UpdateProcessRequest{
		ProcessName:    processName,
		StagedPath:     stagedPath,
		ExpectedSha256: expectedSha256,
	}

	return wrapper.UpdateProcess(ctx, req)
}

func (s *AgentService) RestartProcess(ctx context.Context, agentId, processName string) error {
	if err := s.requireAgentScope(ctx, agentId); err != nil {
		return err
	}
	client := ops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, agentId)
	if err != nil {
		return errors.New("ops client unavailable: " + err.Error())
	}

	req := &opsv1.RestartProcessRequest{
		ProcessName: processName,
	}

	_, err = wrapper.RestartProcess(ctx, req)
	return err
}

// SetSchedulePause suspends or resumes a managed process's schedule firing
// (§10.2 暂停与恢复，暂停对象=调度；进程运行态不动).
func (s *AgentService) SetSchedulePause(ctx context.Context, agentId, processName string, suspended bool) (*opsv1.SchedulePauseResponse, error) {
	if err := s.requireAgentScope(ctx, agentId); err != nil {
		return nil, err
	}
	client := ops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, agentId)
	if err != nil {
		return nil, errors.New("ops client unavailable: " + err.Error())
	}

	req := &opsv1.SchedulePauseRequest{
		ProcessName: processName,
		Suspend:     suspended,
	}

	return wrapper.SetSchedulePause(ctx, req)
}
