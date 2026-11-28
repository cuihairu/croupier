package ops

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/labels"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func opsAgentsList(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentsListRequest) (*OpsAgentsListResponse, error) {
	if svcCtx == nil {
		return &OpsAgentsListResponse{
			Agents: []OpsAgentInfo{},
		}, nil
	}
	store := svcCtx.RegistryStore
	if store == nil {
		return &OpsAgentsListResponse{
			Agents: []OpsAgentInfo{},
		}, nil
	}

	scope := svc.GameScopeFromContext(ctx)

	store.Mu().RLock()
	local := make(map[string]*registry.AgentSession, len(store.AgentsUnsafe()))
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil {
			local[sess.AgentID] = sess
		}
	}
	store.Mu().RUnlock()

	agents := make([]OpsAgentInfo, 0, len(local))

	// 集群模式：以共享归属表为全集（本地 registry 只含连到本实例的
	// agent；只读本地会导致入口分流到不同实例时列表随机变化）。
	if svcCtx.Cluster != nil && svcCtx.Cluster.ListAgentOwners != nil {
		if owners, err := svcCtx.Cluster.ListAgentOwners(ctx); err == nil {
			for _, rec := range owners {
				if strings.TrimSpace(scope.GameID) != "" && rec.GameID != scope.GameID {
					continue
				}
				if strings.TrimSpace(scope.Env) != "" && rec.Env != scope.Env {
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
			attachSupervisorSummaries(svcCtx, agents)
			return &OpsAgentsListResponse{Agents: agents}, nil
		}
		// 归属表读取失败退回本地视图
	}

	// AgentsUnsafe 契约要求持 Mu().RLock——注册/心跳/清理在写锁下改这张
	// map，无锁迭代在写并发时直接 fatal（concurrent map iteration and map
	// write）。上面 :80-87 的锁只覆盖 local 快照，本回退路径原漏锁。
	store.Mu().RLock()
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if strings.TrimSpace(scope.GameID) != "" && sess.GameID != scope.GameID {
			continue
		}
		if strings.TrimSpace(scope.Env) != "" && sess.Env != scope.Env {
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
	store.Mu().RUnlock()

	attachSupervisorSummaries(svcCtx, agents)
	return &OpsAgentsListResponse{
		Agents: agents,
	}, nil
}

func opsAgentMeta(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentMetaRequest) (*OpsAgentMetaResponse, error) {
	if err := NewAgentService(svcCtx).requireAgentScope(ctx, req.AgentId); err != nil {
		return nil, err
	}
	store := svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == req.AgentId {
			return &OpsAgentMetaResponse{
				Meta: OpsAgentSystemInfo{
					OS:       sess.Labels["os"],
					Arch:     sess.Labels["arch"],
					Hostname: sess.Labels["hostname"],
				},
			}, nil
		}
	}

	return nil, errors.New("agent not found")
}

func opsAgentMetrics(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentMetricsRequest) (*OpsAgentMetricsResponse, error) {
	if err := NewAgentService(svcCtx).requireAgentScope(ctx, req.AgentID); err != nil {
		return nil, err
	}
	if svcCtx == nil || svcCtx.MetricsStore == nil {
		return &OpsAgentMetricsResponse{Metrics: []OpsMetricsData{}}, nil
	}
	entry, _ := svcCtx.MetricsStore.GetLatest(req.AgentID)
	if entry == nil || entry.Report == nil {
		return &OpsAgentMetricsResponse{Metrics: []OpsMetricsData{}}, nil
	}
	report := entry.Report
	data := OpsMetricsData{
		AgentID:   req.AgentID,
		Timestamp: utils.FormatTimestamp(entry.Received),
	}
	if cpu := report.GetCpu(); cpu != nil {
		data.CPU = *convertCpuMetrics(cpu)
	}
	if memory := report.GetMemory(); memory != nil {
		data.Memory = *convertMemoryMetrics(memory)
	}
	if len(report.GetDisks()) > 0 {
		data.Disks = convertDiskMetrics(report.GetDisks())
	}
	for _, network := range report.GetNetworks() {
		if network == nil {
			continue
		}
		data.Networks = append(data.Networks, OpsNetworkMetrics{
			Interface:   network.GetInterface(),
			BytesSent:   network.GetBytesSent(),
			BytesRecv:   network.GetBytesRecv(),
			PacketsSent: network.GetPacketsSent(),
			PacketsRecv: network.GetPacketsRecv(),
		})
	}
	return &OpsAgentMetricsResponse{
		Metrics: []OpsMetricsData{data},
	}, nil
}

func opsAgentProcesses(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentProcessesRequest) (*OpsAgentProcessesResponse, error) {
	if err := NewAgentService(svcCtx).requireAgentScope(ctx, req.AgentID); err != nil {
		return nil, err
	}
	if svcCtx == nil || svcCtx.MetricsStore == nil {
		return &OpsAgentProcessesResponse{Processes: []OpsManagedProcess{}}, nil
	}
	entry, _ := svcCtx.MetricsStore.GetLatest(req.AgentID)
	processes := []OpsManagedProcess{}
	if entry != nil && entry.Report != nil {
		for _, snapshot := range entry.Report.GetSupervisedProcesses() {
			if snapshot == nil {
				continue
			}
			processes = append(processes, OpsManagedProcess{
				Name:         snapshot.GetName(),
				State:        supervisorStateString(snapshot.GetState()),
				Pid:          snapshot.GetPid(),
				RestartCount: snapshot.GetRestartCount(),
			})
		}
	}
	return &OpsAgentProcessesResponse{
		Processes: processes,
	}, nil
}

// opsAgentSystemInfo 读 agent 系统信息，四级数据面：SystemInfoCache（离线
// agent 的最后已知信息锚点）→ 隧道 live RPC（回填缓存，§10.5 指纹随
// SystemInfo 上行）→ registry 标签降级视图（隧道不可达时的既有语义，指纹
// 随注册标签在列）→ DB 最后已知锚点（会话过期软删后 registry 内存视图也
// 没有了，agent_sessions 行仍在——指纹随注册标签持久化，跨 server 重启）。
func opsAgentSystemInfo(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentSystemInfoRequest) (*OpsAgentSystemInfoResponse, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	if err := NewAgentService(svcCtx).requireAgentScope(ctx, req.AgentID); err != nil {
		return nil, err
	}

	// 1. 缓存优先：不打扰 agent，最后已知信息（含指纹）即可答复。
	if svcCtx.SystemInfoCache != nil {
		if info, ok := svcCtx.SystemInfoCache.Get(req.AgentID); ok {
			return &OpsAgentSystemInfoResponse{SystemInfo: systemInfoToDTO(info)}, nil
		}
	}

	// 2. 隧道 live RPC。
	info, err := NewAgentService(svcCtx).GetSystemInfo(ctx, req.AgentID)
	if err == nil {
		if svcCtx.SystemInfoCache != nil {
			svcCtx.SystemInfoCache.Set(req.AgentID, info)
		}
		return &OpsAgentSystemInfoResponse{SystemInfo: systemInfoToDTO(info)}, nil
	}
	tunnelErr := err

	// 3. registry 标签降级视图（离线/隧道不可达时的既有行为）。指纹随
	// 注册标签持久化在会话里（§10.5），一并降级可见。
	if store := svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess != nil && sess.AgentID == req.AgentID {
				info := OpsAgentSystemInfo{
					OS:       sess.Labels["os"],
					Arch:     sess.Labels["arch"],
					Hostname: sess.Labels["hostname"],
				}
				if fp := sess.Labels[labels.AgentBinarySHA256]; fp != "" {
					info.AgentBinarySha256 = fp
				}
				if sess.Version != "" {
					info.AgentVersion = sess.Version
				}
				store.Mu().RUnlock()
				return &OpsAgentSystemInfoResponse{SystemInfo: info}, nil
			}
		}
		store.Mu().RUnlock()

		// 4. DB 最后已知锚点：会话过期软删后内存视图没有该 agent，但
		// agent_sessions 行仍在（Unscoped 读，锚点比会话 TTL 活得久）。
		// 查询失败/无记录降级到既有 not found 语义，不放大错误。
		if svcCtx.DB != nil {
			if version, fp, ok, dbErr := registry.NewAgentSessionModel(svcCtx.DB).
				LastKnownIdentity(ctx, req.AgentID); dbErr == nil && ok {
				return &OpsAgentSystemInfoResponse{
					SystemInfo: OpsAgentSystemInfo{
						AgentVersion:      version,
						AgentBinarySha256: fp,
					},
				}, nil
			}
		}

		return nil, errors.New("agent not found")
	}

	return nil, tunnelErr
}

// systemInfoToDTO 把 pb SystemInfo 映射到 API DTO（指纹字段随行）。
func systemInfoToDTO(info *opsv1.SystemInfo) OpsAgentSystemInfo {
	bootTime := ""
	if info.GetBootTime() != nil {
		bootTime = info.GetBootTime().AsTime().Format(time.RFC3339)
	}
	return OpsAgentSystemInfo{
		Hostname:          info.GetHostname(),
		OS:                info.GetOs(),
		OSVersion:         info.GetOsVersion(),
		KernelVersion:     info.GetKernelVersion(),
		Arch:              info.GetArch(),
		CPUCores:          info.GetCpuCores(),
		TotalMemory:       info.GetTotalMemory(),
		BootTime:          bootTime,
		AgentVersion:      info.GetAgentVersion(),
		AgentBinarySha256: info.GetAgentBinarySha256(),
	}
}

// opsAgentExecCommand 在 Agent 上执行命令（高风险动作）。§10.7 远程 Shell
// 边界审计前置：每次尝试（成功 / 业务失败 / 客户端不可用）都写 execution_logs，
// 命令原文与结果摘要入审计载荷；审计面缺失（无 Writer）不阻断执行主路径。
func opsAgentExecCommand(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsExecCommandRequest) (*OpsExecCommandResponse, error) {
	agentSvc := NewAgentService(svcCtx)
	startedAt := time.Now()
	result, err := agentSvc.ExecCommand(ctx, req.AgentID, req.Command, req.Args, int(req.Timeout))
	if err != nil {
		auditAgentExec(ctx, svcCtx, req, startedAt, nil, err)
		return nil, err
	}

	auditAgentExec(ctx, svcCtx, req, startedAt, result, nil)
	return &OpsExecCommandResponse{
		Result: OpsExecCommandResult{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		},
	}, nil
}

// auditAgentExec 写 exec 审计留痕（fail-open：只投递不阻塞，Writer 缺失跳过）。
func auditAgentExec(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsExecCommandRequest, startedAt time.Time, result *OpsExecCommandResult, execErr error) {
	if svcCtx == nil || svcCtx.ExecutionLogWriter == nil {
		return
	}
	actor := "unknown"
	if name, err := utils.CurrentUsername(ctx); err == nil {
		actor = name
	}
	scope := svc.GameScopeFromContext(ctx)

	status := executionlog.StatusOK
	var respBody interface{}
	if execErr != nil {
		status = executionlog.StatusFail
		respBody = map[string]string{"error": execErr.Error()}
	} else if result != nil {
		respBody = map[string]interface{}{
			"exitCode": result.ExitCode,
			"stdout":   result.Stdout,
			"stderr":   result.Stderr,
		}
	}

	svcCtx.ExecutionLogWriter.Log(executionlog.Entry{
		GameID:     scope.GameID,
		Env:        scope.Env,
		Source:     executionlog.SourceAgentExec,
		FunctionID: req.Command,
		Actor:      actor,
		Route:      "/ops/agents/" + req.AgentID + "/exec",
		Status:     status,
		DurationMs: time.Since(startedAt).Milliseconds(),
		Request: map[string]interface{}{
			"agentId": req.AgentID,
			"command": req.Command,
			"args":    req.Args,
			"timeout": req.Timeout,
		},
		Response: respBody,
	})
}

// opsAgentProcessUpdate 在 Agent 上换入受管进程新二进制（§10.4 更新状态）。
// §10.4 指纹硬要求：每次尝试（成功 / 失败 / 回滚）写 execution_logs（source=
// process_update），新旧指纹与回滚留痕入审计载荷——持久查询主入口按
// 「目标 + 时间 + 指纹」检索本表。
func opsAgentProcessUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessUpdateRequest) (*OpsProcessUpdateResponse, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	agentSvc := NewAgentService(svcCtx)
	startedAt := time.Now()
	result, err := agentSvc.UpdateProcess(ctx, req.AgentID, strings.TrimSpace(req.Name), strings.TrimSpace(req.StagedPath), strings.TrimSpace(req.ExpectedSha256))
	if err != nil {
		auditAgentUpdate(ctx, svcCtx, req, startedAt, nil, err)
		return nil, err
	}

	auditAgentUpdate(ctx, svcCtx, req, startedAt, result, nil)
	return &OpsProcessUpdateResponse{
		Success:        result.GetSuccess(),
		Message:        result.GetMessage(),
		OldFingerprint: result.GetOldFingerprint(),
		NewFingerprint: result.GetNewFingerprint(),
		RolledBack:     result.GetRolledBack(),
	}, nil
}

// auditAgentUpdate 写更新审计留痕（fail-open：Writer 缺失跳过，不阻断主路径）。
func auditAgentUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessUpdateRequest, startedAt time.Time, result *opsv1.UpdateProcessResponse, updateErr error) {
	if svcCtx == nil || svcCtx.ExecutionLogWriter == nil {
		return
	}
	actor := "unknown"
	if name, err := utils.CurrentUsername(ctx); err == nil {
		actor = name
	}
	scope := svc.GameScopeFromContext(ctx)

	status := executionlog.StatusOK
	var respBody interface{}
	if updateErr != nil {
		status = executionlog.StatusFail
		respBody = map[string]string{"error": updateErr.Error()}
	} else if result != nil {
		respBody = map[string]interface{}{
			"success":        result.GetSuccess(),
			"oldFingerprint": result.GetOldFingerprint(),
			"newFingerprint": result.GetNewFingerprint(),
			"rolledBack":     result.GetRolledBack(),
			"message":        result.GetMessage(),
		}
	}

	svcCtx.ExecutionLogWriter.Log(executionlog.Entry{
		GameID:     scope.GameID,
		Env:        scope.Env,
		Source:     executionlog.SourceProcessUpdate,
		FunctionID: req.Name,
		Actor:      actor,
		Route:      "/ops/agents/" + req.AgentID + "/processes/" + req.Name + "/update",
		Status:     status,
		DurationMs: time.Since(startedAt).Milliseconds(),
		Request: map[string]interface{}{
			"agentId":        req.AgentID,
			"process":        req.Name,
			"stagedPath":     req.StagedPath,
			"expectedSha256": strings.ToLower(strings.TrimSpace(req.ExpectedSha256)),
		},
		Response: respBody,
	})
}

func opsAgentProcessStart(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessStartRequest) (*OpsProcessStartResponse, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	agentSvc := NewAgentService(svcCtx)
	pid, err := agentSvc.StartProcess(ctx, req.AgentID, req.Name, nil, nil, "")
	if err != nil {
		return nil, err
	}
	return &OpsProcessStartResponse{
		Pid: int32(pid),
	}, nil
}

func opsAgentProcessStop(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessActionRequest) (*OpsProcessActionResponse, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	agentSvc := NewAgentService(svcCtx)
	processName := strings.TrimSpace(req.Name)
	if err := agentSvc.StopProcess(ctx, req.AgentID, processName); err != nil {
		return nil, err
	}
	return &OpsProcessActionResponse{}, nil
}

func opsAgentProcessRestart(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessActionRequest) (*OpsProcessActionResponse, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	agentSvc := NewAgentService(svcCtx)
	processName := strings.TrimSpace(req.Name)
	if err := agentSvc.RestartProcess(ctx, req.AgentID, processName); err != nil {
		return nil, err
	}
	return &OpsProcessActionResponse{}, nil
}

func opsAgentProcessSchedulePause(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessSchedulePauseRequest) (*OpsProcessSchedulePauseResponse, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	agentSvc := NewAgentService(svcCtx)
	processName := strings.TrimSpace(req.Name)
	resp, err := agentSvc.SetSchedulePause(ctx, req.AgentID, processName, req.Suspend)
	if err != nil {
		return nil, err
	}
	return &OpsProcessSchedulePauseResponse{
		Success:   resp.GetSuccess(),
		Message:   resp.GetMessage(),
		Suspended: resp.GetSuspended(),
	}, nil
}

// Backup operations implementations
