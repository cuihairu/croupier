package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cuihairu/croupier/internal/common/labels"
	versionutil "github.com/cuihairu/croupier/internal/common/version"
	"github.com/cuihairu/croupier/internal/function/converter"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/cuihairu/croupier/internal/function/schemadiff"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/registry/sdkversion"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"google.golang.org/protobuf/proto"
)

// maxAgentIDLen bounds the agent identifier to the agent_id columns' schema
// size (VARCHAR(64), matching agent_sessions/agent_metrics_hourly/etc.).
// VARCHAR(64) limits characters, not bytes, so the guard uses rune count.
// Registration accepts arbitrary-length IDs, but the agent_id columns are
// size:64, so a strict database (PostgreSQL) rejects an over-long ID at write
// or migration time. Rejecting at the register entry keeps over-long IDs out
// of every store and keeps migration 0063 from failing on existing rows.
const maxAgentIDLen = 64

type registerWarning struct {
	Code       string
	FunctionID string
	Version    string
	Message    string
}

// ListenAddr represents a single listen address with transport type.
type ListenAddr struct {
	Addr      string // Raw address (e.g., ":19090", "ipc://croupier-server")
	Transport string // Transport type: "tcp", "ipc", etc.
	URL       string // Full URL (e.g., "tcp://:19090", "ipc://croupier-server")
}

// ParseListenAddr parses a string address into a ListenAddr.
func ParseListenAddr(addr string) ListenAddr {
	if strings.Contains(addr, "://") {
		parts := strings.SplitN(addr, "://", 2)
		return ListenAddr{
			Addr:      parts[1],
			Transport: parts[0],
			URL:       addr,
		}
	}
	return ListenAddr{
		Addr:      addr,
		Transport: "tcp",
		URL:       "tcp://" + addr,
	}
}

// IsLocalTCP checks if an address is a local TCP address.
func IsLocalTCP(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func (s *ControlService) handleRegister(ctx context.Context, data []byte) ([]byte, error) {
	req := &agentv1.RegisterRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterRequest: %w", err)
	}
	resp, err := s.handleRegisterRequest(ctx, req, "")
	if err != nil {
		return nil, err
	}
	return proto.Marshal(resp)
}

func (s *ControlService) handleHeartbeat(ctx context.Context, data []byte) ([]byte, error) {
	req := &agentv1.HeartbeatRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal HeartbeatRequest: %w", err)
	}
	resp, err := s.handleHeartbeatRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(resp)
}

func (s *ControlService) handleRegisterCapabilities(ctx context.Context, data []byte) ([]byte, error) {
	req := &agentv1.RegisterCapabilitiesRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterCapabilitiesRequest: %w", err)
	}
	resp, err := s.handleRegisterCapabilitiesRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(resp)
}

// handleRegisterRequest implements the actual Register logic.
func (s *ControlService) handleRegisterRequest(ctx context.Context, req *agentv1.RegisterRequest, remoteAddr string) (*agentv1.RegisterResponse, error) {
	if s.upstream != nil {
		return s.upstream.HandleRegister(ctx, req)
	}

	ttl := 24 * time.Hour
	if req.TtlSeconds > 0 {
		ttl = time.Duration(req.TtlSeconds) * time.Second
	}

	agentID := strings.TrimSpace(req.GetAgentId())
	gameID := strings.TrimSpace(req.GetGameId())
	env := strings.TrimSpace(req.GetEnv())
	if agentID == "" || gameID == "" || env == "" {
		return nil, fmt.Errorf("agent_id, game_id and env are required")
	}
	if utf8.RuneCountInString(agentID) > maxAgentIDLen {
		return nil, fmt.Errorf("agent_id too long: max %d chars, got %d", maxAgentIDLen, utf8.RuneCountInString(agentID))
	}
	if previous := s.registry.PreviousAgentSession(agentID); previous != nil {
		if previous.GameID != gameID || previous.Env != env {
			return nil, fmt.Errorf("agent %q is already bound to game_id=%q env=%q", agentID, previous.GameID, previous.Env)
		}
	}
	req.AgentId = agentID
	req.GameId = gameID
	req.Env = env

	functions, warnings := validateAndNormalizeFunctions(req.GetFunctions())
	warningTexts := make([]string, 0, len(warnings))
	for _, warnMsg := range warnings {
		warningTexts = append(warningTexts, warnMsg.Message)
		s.logger.Warn("register validation warning", "agent_id", req.AgentId, "warning", warnMsg.Message, "code", warnMsg.Code, "function_id", warnMsg.FunctionID, "version", warnMsg.Version)
		s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
			GameID:     req.GameId,
			Env:        req.Env,
			AgentID:    req.AgentId,
			FunctionID: warnMsg.FunctionID,
			Version:    warnMsg.Version,
			Code:       warnMsg.Code,
			Message:    warnMsg.Message,
		})
	}

	// SDK 滑动版本门槛（针对函数注册）：按 (game_id, env, sdk_language) 的
	// 历史最高 SDK 版本评估各 provider 进程；落后超容忍窗（低 2 个及以上
	// minor 或 1 个及以上 major）的进程其函数不进本次注册。判定在
	// sess.Functions 构造之前完成；抬升高水位在 UpsertAgent 成功之后。
	sdkFloor := s.evaluateSDKVersionFloor(ctx, req, &warningTexts)

	sess := &reg.AgentSession{
		AgentID: req.AgentId,
		GameID:  req.GameId,
		Env:     req.Env,
		Addr:    remoteAddr,
		Version: req.Version,
		Region:  "",
		Zone:    "",
		// 保留 agent 自报系统标签（os/arch/hostname/ip 等）：IP 列显示的是
		// server 看到的 TCP 对端（经 LB 时全为 LB 地址），自报 ip/hostname
		// 是区分 agent 的关键信息。心跳路径的 reportedOwner 在后续心跳写入。
		Labels:    sanitizeAgentLabels(req.GetLabels()),
		ExpireAt:  time.Now().Add(ttl),
		LastSeen:  time.Now(),
		Functions: map[string]reg.FunctionMeta{},
	}

	// validateAndNormalizeFunctions 的后置条件：输出切片中每个 f 非 nil 且
	// Id 为 trim+lower 后非空、通过 functionIDPattern 校验的值（nil 与
	// 空 Id 在该函数内已全部 continue 剔除并转为 warning），故此处原
	// `if f == nil || f.Id == "" { continue }` 防御分支为死代码，已删。
	for _, f := range functions {
		if sdkFloor.rejected[f.Id] {
			continue
		}
		sess.Functions[f.Id] = reg.FunctionMeta{
			Enabled:           f.Enabled,
			Version:           f.Version,
			Tags:              append([]string(nil), f.Tags...),
			Summary:           f.GetSummary(),
			Description:       f.GetDescription(),
			OperationID:       f.Id,
			Deprecated:        f.GetDeprecated(),
			InputSchema:       f.GetInputSchema(),
			OutputSchema:      f.GetOutputSchema(),
			Resource:          f.GetResource(),
			Operation:         f.GetOperation(),
			Capability:        f.GetCapability(),
			Execution:         f.GetExecution(),
			ApprovalRequired:  f.GetApprovalRequired(),
			ApprovalPolicyKey: f.GetApprovalPolicyKey(),
			Risk:              f.GetRisk(),
			Permission:        f.GetPermission(),
		}
		if op, err := converter.ToOpenAPIOperation(converter.ProviderFunctionDescriptorDesc{
			ID:           f.Id,
			Version:      f.Version,
			Tags:         f.Tags,
			Summary:      f.GetSummary(),
			Description:  f.GetDescription(),
			OperationID:  f.Id,
			Deprecated:   f.GetDeprecated(),
			InputSchema:  f.GetInputSchema(),
			OutputSchema: f.GetOutputSchema(),
			Resource:     f.GetResource(),
			Operation:    f.GetOperation(),
			Capability:   f.GetCapability(),
			Execution:    f.GetExecution(),
			Risk:         f.GetRisk(),
			Permission:   f.GetPermission(),
		}); err == nil {
			// UpsertOpenAPI 的错误在合法输入域不可达（C 类）：functionID 非空
			// 由 validateAndNormalizeFunctions 后置条件保证（空 Id 被 skip）；op 非
			// nil 由 converter 成功路径恒返回字面量构造保证；cloneOpenAPIOperation
			// 的 MarshalJSON 对「Unmarshal 成功产物」恒成功——与 versioning 曾误删
			// 的场景（未校验的 JSONSchema 原始文本直接透传进 Marshal，可被
			// "{invalid" 击穿）本质不同：非法 InputSchema 在 converter 的第一层
			// json.Unmarshal 即被拒绝（走下方 else Warn 分支），进入 clone 的
			// Schema 是 kin-openapi 类型化字段 + Extensions(仅 string 字面量
			// x-resource/x-risk 等)，json.Unmarshal 产出的值类型集合对 Marshal
			// 无条件可序列化。已用 28 组恶意 InputSchema（2000 层深嵌套、lone
			// surrogate、任意 x- 扩展 map、大整数、混合 enum、$ref 等）实证
			// 无法构造 UpsertOpenAPI 失败。该 Warn 为防御性错误处理保留，
			// 经 upsertOpenAPI 注入点测试驱动。
			upsert := s.upsertOpenAPI
			if upsert == nil {
				upsert = s.registry.UpsertOpenAPI
			}
			if err := upsert(f.Id, op); err != nil {
				s.logger.Warn("failed to upsert openapi operation from register request", "function_id", f.Id, "error", err)
			}
		} else {
			s.logger.Warn("failed to convert function descriptor to openapi operation", "function_id", f.Id, "error", err)
		}
	}

	// F12：注册时 schema 兼容性检查——与会话中上一次注册的 schema 对比，
	// 破坏性变更写入注册警告并随 RegisterResponse.warnings 返回 agent。
	// 只告警不阻断；必须在 UpsertAgent 覆盖会话之前执行。
	if s.schemaDiffWarn {
		// 同上：functions 经 validateAndNormalizeFunctions 后置条件保证非 nil
		// 且 Id 非空，原防御分支为死代码，已删。被门槛拒绝的函数不进本次
		// 注册，其 schema 对比是无意义的噪音，跳过。
		for _, f := range functions {
			if sdkFloor.rejected[f.Id] {
				continue
			}
			oldInput, oldOutput, ok := s.registry.PreviousFunctionSchema(req.AgentId, req.GameId, req.Env, f.Id)
			if !ok {
				continue
			}
			findings := schemadiff.DiffSchemas("inputSchema", json.RawMessage(oldInput), json.RawMessage(f.GetInputSchema()))
			findings = append(findings,
				schemadiff.DiffSchemas("outputSchema", json.RawMessage(oldOutput), json.RawMessage(f.GetOutputSchema()))...)
			for _, finding := range findings {
				if finding.Severity != schemadiff.SeverityBreaking {
					continue
				}
				warning := fmt.Sprintf("function %s %s%s: %s", f.Id, finding.Source, finding.Path, finding.Reason)
				warningTexts = append(warningTexts, warning)
				s.logger.Warn("register schema breaking change", "agent_id", req.AgentId, "function_id", f.Id, "source", finding.Source, "path", finding.Path, "reason", finding.Reason)
				s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
					GameID:     req.GameId,
					Env:        req.Env,
					AgentID:    req.AgentId,
					FunctionID: f.Id,
					Version:    f.Version,
					Code:       "schema_breaking_change",
					Message:    warning,
				})
			}
		}
	}

	if len(req.Processes) > 0 {
		providers := make([]reg.ProviderSession, 0, len(req.Processes))
		for _, p := range req.Processes {
			if p == nil || p.ServiceId == "" {
				continue
			}
			// Provider（SDK/自定义游戏服）scope 校验（作用域规范 §14.5）：
			// agent 会话绑定单一 (game_id, env)，provider 上报的 scope 必须
			// 一致——空值同样视为 mismatch，无兼容分支。不一致说明 SDK 侧
			// 配置错误：产生业务层注册警告（路由仍按 agent scope 保持稳定）。
			s.validateProviderScope(ctx, req, p, &warningTexts)
			// provider 会话版本与函数槽同门槛：非 semver（含空/"unknown"/误传
			// providerID）不进 version 槽——函数实例页该列即「契约版本」，
			// 脏值直通会让下游 sdkversion gate/排障全部失真。
			providerVersion := strings.TrimSpace(p.Version)
			if !versionutil.IsValid(providerVersion) {
				normalized := versionutil.ValidOrDefault(providerVersion)
				if providerVersion != "" {
					warningTexts = append(warningTexts, fmt.Sprintf("provider_version_invalid: service=%s version=%q is not valid semver; normalized to %s", p.ServiceId, p.Version, normalized))
				}
				providerVersion = normalized
			}
			providers = append(providers, reg.ProviderSession{
				ProviderID:   p.ServiceId,
				GameID:       req.GameId,
				Env:          req.Env,
				Addr:         p.Addr,
				Version:      providerVersion,
				SDKLanguage:  p.SdkLanguage,
				SDKVersion:   p.SdkVersion,
				SDKName:      p.SdkName,
				Metadata:     p.GetMetadata(),
				LastSeenUnix: p.LastSeenUnix,
				FunctionIDs:  p.FunctionIds,
			})
		}
		sess.Providers = providers
	}

	if err := s.registry.UpsertAgent(sess); err != nil {
		return nil, fmt.Errorf("register agent dashboard contract rebuild failed: %w", err)
	}

	// 门槛放行的 provider 注册成功后抬升 (game_id, env, sdk_language) 高水位，
	// 作为后续注册的滑动下限。放行 ≠ 抬升：低版本放行（容忍窗内）不抬；
	// 不可解析版本（"unknown"）不抬。
	for _, raise := range sdkFloor.raises {
		s.registry.ObserveSDKVersion(req.GameId, req.Env, raise.language, raise.version)
	}

	// 集群归属：本实例持有该 Agent 连接（多实例 HA 转发路由依据）。
	s.mu.RLock()
	hooks := s.clusterHooks
	s.mu.RUnlock()
	if hooks != nil {
		hooks.OnAgentRegistered(ctx, req.AgentId, req.GameId, req.Env, sess.Labels)
	}

	// NewStoreWithDB persists the session only after contract/proposal
	// materialization succeeds. Do not duplicate that write through the loader;
	// the loader remains the persistence path for memory-only registries and is
	// still used by heartbeat/expiry maintenance.
	if s.agentSessionLoader != nil && !s.registry.SessionPersistenceEnabled() {
		if err := s.agentSessionLoader.Upsert(ctx, sess); err != nil {
			s.logger.Error("failed to write agent session to database", "agent_id", req.AgentId, "error", err)
		}
	}

	s.logger.Info("Agent registered", "agent_id", req.AgentId, "game_id", req.GameId, "functions", len(functions), "warnings", len(warnings))

	s.mu.RLock()
	selfID := s.clusterInstanceID
	s.mu.RUnlock()
	return &agentv1.RegisterResponse{
		Warnings:   warningTexts,
		InstanceId: selfID,
	}, nil
}

func (s *ControlService) handleHeartbeatRequest(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	if s.upstream != nil {
		return s.upstream.HandleHeartbeat(ctx, req)
	}

	if req.AgentId == "" {
		return &agentv1.HeartbeatResponse{}, nil
	}

	s.registry.Mu().Lock()
	agent := s.registry.AgentsUnsafe()[req.AgentId]
	// agent 自报 owner（三方对账，agent 视角）：存入 session labels 供
	// nodes/cluster 视图与归属表 claim、LB 会话对照（半开/漂移探测）。
	if agent != nil && req.OwnerInstanceId != "" {
		if agent.Labels == nil {
			agent.Labels = map[string]string{}
		}
		agent.Labels["reportedOwner"] = req.OwnerInstanceId
	}
	if agent == nil {
		s.registry.Mu().Unlock()
		// 僵尸防线：会话在本地 registry 意外丢失（过期清理/替换竞态）但
		// TCP 连接仍活——心跳不能再静默成功，否则归属行冻结、agent 永不
		// 自愈。从共享归属表回读真实 scope 重建最小会话并重新 Claim。
		var gameID, env string
		var owns bool
		s.mu.RLock()
		lookup := s.heartbeatOwnerLookup
		s.mu.RUnlock()
		if lookup != nil {
			gameID, env, owns = lookup.SelfOwnerScope(ctx, req.AgentId)
		}
		if !owns {
			s.logger.Error("heartbeat for unknown agent session; cannot self-heal (not this instance's claim or no owner record)", "agent_id", req.AgentId)
			return &agentv1.HeartbeatResponse{}, nil
		}
		ttl := s.defaultSessionTTL
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
		reseed := &reg.AgentSession{
			AgentID:   req.AgentId,
			GameID:    gameID,
			Env:       env,
			ExpireAt:  time.Now().Add(ttl),
			LastSeen:  time.Now(),
			Labels:    map[string]string{"reseeded": "true"},
			Functions: map[string]reg.FunctionMeta{},
			Providers: []reg.ProviderSession{},
		}
		if err := s.registry.UpsertAgent(reseed); err != nil {
			s.logger.Error("heartbeat self-heal reseed failed", "agent_id", req.AgentId, "error", err)
			return &agentv1.HeartbeatResponse{}, nil
		}
		s.mu.RLock()
		hooks := s.clusterHooks
		s.mu.RUnlock()
		if hooks != nil {
			hooks.OnAgentRegistered(ctx, req.AgentId, gameID, env, reseed.Labels)
		}
		s.logger.Warn("heartbeat self-healed: session was missing, re-seeded from ownership table",
			"agent_id", req.AgentId, "game_id", gameID, "env", env)
		return &agentv1.HeartbeatResponse{}, nil
	}
	if agent != nil {
		agent.ExpireAt = time.Now().Add(s.defaultSessionTTL)
		agent.LastSeen = time.Now()
		s.registry.Mu().Unlock()
		s.mu.RLock()
		hooks := s.clusterHooks
		s.mu.RUnlock()
		if hooks != nil {
			hooks.OnAgentHeartbeat(ctx, req.AgentId)
		}
		s.registry.Mu().Lock()
		if s.agentSessionLoader != nil && s.shouldPersistSession(req.AgentId) {
			// 快照后再异步落盘：agent 指针仍挂在 registry 上，后续心跳会在
			// registry 锁内继续写 ExpireAt/LastSeen 与 Labels["reportedOwner"]，
			// 异步 Upsert 直接读活动指针构成 data race。
			agentToUpdate := snapshotAgentSession(agent)
			go func() {
				if err := s.agentSessionLoader.Upsert(context.Background(), agentToUpdate); err != nil {
					s.logger.Error("failed to update agent session in database", "agent_id", req.AgentId, "error", err)
				}
			}()
		}
	}
	s.registry.Mu().Unlock()

	return &agentv1.HeartbeatResponse{}, nil
}

// sanitizeAgentLabels 复制 agent 注册时上报的系统标签。限制数量与键值
// 长度，防止注册路径被塞入任意大的标签集合；空键/空值丢弃，超长值截断。
// 平台契约键（agentBinarySha256）豁免数量上限：map 迭代次序随机，满额
// break 会按运气静默丢弃指纹锚点的唯一数据源——限额约束的是用户自选
// 标签量，平台键固定 1 个、不占额度。
func sanitizeAgentLabels(agentLabels map[string]string) map[string]string {
	const (
		maxLabels = 32
		maxKeyLen = 64
		maxValLen = 256
	)
	sysVal := ""
	if fp := strings.TrimSpace(agentLabels[labels.AgentBinarySHA256]); fp != "" {
		if len(fp) > maxValLen {
			fp = fp[:maxValLen]
		}
		sysVal = fp
	}
	out := make(map[string]string, min(len(agentLabels), maxLabels))
	for k, v := range agentLabels {
		if k == labels.AgentBinarySHA256 {
			continue // 平台键走豁免通道，不占限额也不受随机次序影响
		}
		if len(out) >= maxLabels {
			break
		}
		k = strings.TrimSpace(k)
		if k == "" || len(k) > maxKeyLen {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if len(v) > maxValLen {
			v = v[:maxValLen]
		}
		out[k] = v
	}
	if sysVal != "" {
		out[labels.AgentBinarySHA256] = sysVal
	}
	return out
}

// snapshotAgentSession 在 registry 锁内拷贝会话快照（异步落盘用）。
// 值拷贝覆盖标量与 map 引用：Functions/Providers 是替换语义（重新注册
// 换新 map，旧 map 不再被写），引用拷贝即安全；Labels 是原地 merge
// 语义（心跳写 reportedOwner、注册 merge 标签），必须深拷贝。
func snapshotAgentSession(a *reg.AgentSession) *reg.AgentSession {
	if a == nil {
		return nil
	}
	snap := *a
	if a.Labels != nil {
		snap.Labels = make(map[string]string, len(a.Labels))
		for k, v := range a.Labels {
			snap.Labels[k] = v
		}
	}
	return &snap
}

// shouldPersistSession 心跳落盘节流：间隔内只落一次（注册路径不受限）。
func (s *ControlService) shouldPersistSession(agentID string) bool {
	s.sessionPersistMu.Lock()
	defer s.sessionPersistMu.Unlock()
	now := time.Now()
	if s.sessionPersistInterval <= 0 {
		s.sessionPersistInterval = time.Minute
	}
	if s.sessionPersistAt == nil {
		s.sessionPersistAt = map[string]time.Time{}
	}
	if last, ok := s.sessionPersistAt[agentID]; ok && now.Sub(last) < s.sessionPersistInterval {
		return false
	}
	s.sessionPersistAt[agentID] = now
	return true
}

// validateProviderScope 校验 provider 上报的 game/env 与 agent 会话 scope
// 是否一致。警告只进业务层注册警告（/system/functions/warnings，研发
// 视角）——分层原则：业务层（接入契约）与支持层（运维告警，资源/
// 基础设施）分开，运维不关注业务接入问题，研发不关注支持层信息。
// 修复闭环同样在业务层：一致注册时清除该 provider 的历史 mismatch
// 警告（生命周期跟随注册行为）。硬切无兼容：空 scope 即 mismatch。
func (s *ControlService) validateProviderScope(ctx context.Context, req *agentv1.RegisterRequest, p *agentv1.AgentProcess, warningTexts *[]string) {
	// 硬切无兼容（作用域规范 §14）：provider 必须上报 scope 且与 agent
	// 会话一致——空值同样视为 mismatch（SDK 必须显式携带 scope）。
	mismatches := make([]string, 0, 2)
	if p.GameId != req.GameId {
		mismatches = append(mismatches, fmt.Sprintf("game_id mismatch: provider=%q agent=%q", p.GameId, req.GameId))
	}
	if p.Env != req.Env {
		mismatches = append(mismatches, fmt.Sprintf("env mismatch: provider=%q agent=%q", p.Env, req.Env))
	}

	if len(mismatches) == 0 {
		// 一致注册：清除该 agent 的历史 scope mismatch 警告（修复闭环）。
		s.registry.RemoveRegistrationWarnings(reg.RegistrationWarningFilter{
			GameID:  req.GameId,
			Env:     req.Env,
			AgentID: req.AgentId,
			Code:    "provider_scope_mismatch",
		})
		return
	}

	msg := fmt.Sprintf("service_id=%s: %s", p.ServiceId, strings.Join(mismatches, "; "))
	*warningTexts = append(*warningTexts, msg)
	s.logger.Warn("provider scope mismatch", "agent_id", req.AgentId, "service_id", p.ServiceId, "detail", msg)
	s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
		GameID:  req.GameId,
		Env:     req.Env,
		AgentID: req.AgentId,
		Code:    "provider_scope_mismatch",
		Message: msg,
	})
}

// sdkFloorRaise 是一次通过门槛且需要抬升高水位的版本观测。
type sdkFloorRaise struct {
	language string
	version  string
}

// sdkFloorOutcome 是一次注册请求的 SDK 滑动版本门槛评估结果：
// rejected 为被拒函数集合（不进 sess.Functions、不物化契约），raises 为
// 注册成功后待抬升的 (language, version) 观测。
type sdkFloorOutcome struct {
	rejected map[string]bool
	raises   []sdkFloorRaise
}

// evaluateSDKVersionFloor 两道判定：
//
//  1. 函数级最低函数版本（function_version_floors）：比描述符自身
//     version，旧版函数声明不物化（防契约回退），逐函数告警；
//
//  2. 按 (game_id, env, sdk_language) 的历史最高 SDK 版本（高水位）
//     评估注册请求中的 provider 进程（sdkversion 三档判定）：
//
//     - 配置了 registry.sdkVersionMinimums 的语言，自报版本低于配置值
//     的进程直接拒注册（绝对下限，优先于高水位判定，无高水位也生效）；
//     - 无 sdk_language/sdk_version 自报的进程（自定义游戏服直连）不参与
//     门槛，函数照常放行且不抬升高水位；
//     - 版本解析失败的进程一律放行（门槛只在两端都可解析时生效）；
//     - 落后超容忍窗（低 2 个及以上 minor 或 1 个及以上 major）的进程，
//     其独占声明的函数从本次注册中剔除——多个进程交叉提供同一函数时，
//     只要任一放行进程也声明了该函数就不剔（避免误伤），即使其声明者
//     部分被拒。
//
// 剔除只作用于函数面（sess.Functions 与物化路径）；provider 进程记录与
// 其自报 function_ids 保留在会话中（进程存在是事实，调度候选第一道门
// 是 agent.Functions，不会误路由）。连接保持，拒绝以注册警告 +
// RegisterResponse.warnings 传达 agent 日志。
func (s *ControlService) evaluateSDKVersionFloor(ctx context.Context, req *agentv1.RegisterRequest, warningTexts *[]string) sdkFloorOutcome {
	outcome := sdkFloorOutcome{rejected: map[string]bool{}}
	rejectedByProc := map[string][]string{} // service_id -> 被拒声明的函数
	allowedDeclared := map[string]bool{}    // 任一放行进程声明的函数
	// 函数级最低版本门槛（function_version_floors 表，UI 按函数配置）：
	// 批量取一次，逐函数判定——低于该函数配置值的声明单独被拒（不影响
	// 同进程其他达标函数），优先级高于语言级 yaml 与高水位（后两者是
	// 进程级判定，函数级在放行路径上再叠一层）。未自报版本的进程
	// （SdkVersion 为空）Below 恒 false，不受函数级门槛限制（保守一致）。
	// 函数级最低版本门槛（function_version_floors 表，UI 按函数配置）：
	// 比的是函数描述符自身的版本，与 SDK 版本无关——滚动升级窗口里
	// 旧 game server 的重注册会把物化契约刷回旧版（契约回退/振荡），
	// 低于配置值的旧版函数声明不物化（只产生告警），契约永不回退。
	// 版本不可解析（空/"unknown"）不触发（保守一致，Below 恒 false）。
	fnFloors := s.registry.GetFunctionVersionFloors(req.GameId, req.Env)
	for _, f := range req.Functions {
		if f == nil || f.Id == "" {
			continue
		}
		min, ok := fnFloors[f.Id]
		if !ok || min == "" {
			continue
		}
		if sdkversion.Below(f.Version, min) {
			outcome.rejected[f.Id] = true
			msg := fmt.Sprintf("function_id=%s version=%s below configured minimum %s: function registration rejected", f.Id, f.Version, min)
			*warningTexts = append(*warningTexts, msg)
			s.logger.Warn("function version floor rejected registration", "agent_id", req.AgentId, "function_id", f.Id, "function_version", f.Version, "configured_minimum", min)
			s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
				GameID:     req.GameId,
				Env:        req.Env,
				AgentID:    req.AgentId,
				FunctionID: f.Id,
				Version:    f.Version,
				Code:       reg.WarningCodeFunctionVersionBelowMinimum,
				Message:    msg,
			})
		} else {
			// 修复闭环（生命周期跟随注册行为，对齐 provider_scope_mismatch）：
			// 达标版本注册成功即清该函数的历史拦截警告——执行侧门槛拦截
			// 判定（dispatcher.noLiveAgentError）依赖「警告存在 ⟹ 最近注册
			// 仍被拦」，不清会把已恢复函数误述成被门槛拦截。
			s.registry.RemoveRegistrationWarnings(reg.RegistrationWarningFilter{
				GameID:     req.GameId,
				Env:        req.Env,
				FunctionID: f.Id,
				Code:       reg.WarningCodeFunctionVersionBelowMinimum,
			})
		}
	}
	if len(req.Processes) == 0 {
		return outcome
	}
	for _, p := range req.Processes {
		if p == nil || p.ServiceId == "" {
			continue
		}
		language := strings.TrimSpace(p.SdkLanguage)
		if language == "" || strings.TrimSpace(p.SdkVersion) == "" {
			for _, fid := range p.FunctionIds {
				allowedDeclared[fid] = true
			}
			continue
		}
		// 配置最低版本门槛（registry.sdkVersionMinimums）：绝对下限，
		// 优先于滑动高水位——即使尚无高水位观测，低于配置值也拒注册。
		// 版本解析失败不触发（Below 恒 false，与高水位门槛同样保守）。
		if minimum, ok := s.sdkVersionMinimums[strings.ToLower(language)]; ok && sdkversion.Below(p.SdkVersion, minimum) {
			rejectedByProc[p.ServiceId] = p.FunctionIds
			msg := fmt.Sprintf("service_id=%s sdk=%s/%s below configured minimum %s: function registration rejected (%d functions)", p.ServiceId, language, p.SdkVersion, minimum, len(p.FunctionIds))
			*warningTexts = append(*warningTexts, msg)
			s.logger.Warn("sdk version below configured minimum", "agent_id", req.AgentId, "service_id", p.ServiceId, "sdk_language", language, "sdk_version", p.SdkVersion, "configured_minimum", minimum, "functions", len(p.FunctionIds))
			s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
				GameID:  req.GameId,
				Env:     req.Env,
				AgentID: req.AgentId,
				Code:    reg.WarningCodeSDKVersionBelowMinimum,
				Message: msg,
			})
			continue
		}
		floor := s.registry.GetSDKVersionFloor(req.GameId, req.Env, language)
		switch sdkversion.Compare(p.SdkVersion, floor) {
		case sdkversion.VerdictAllow:
			for _, fid := range p.FunctionIds {
				allowedDeclared[fid] = true
			}
			// 不可解析版本 Compare 恒 Allow，但高水位只记可解析版本。
			if sdkversion.Parseable(p.SdkVersion) {
				outcome.raises = append(outcome.raises, sdkFloorRaise{language: language, version: p.SdkVersion})
			}
		case sdkversion.VerdictAllowWithWarning:
			for _, fid := range p.FunctionIds {
				allowedDeclared[fid] = true
			}
			msg := fmt.Sprintf("service_id=%s sdk=%s/%s behind high watermark %s: registration accepted, upgrade the SDK", p.ServiceId, language, p.SdkVersion, floor)
			*warningTexts = append(*warningTexts, msg)
			s.logger.Warn("sdk version behind high watermark", "agent_id", req.AgentId, "service_id", p.ServiceId, "sdk_language", language, "sdk_version", p.SdkVersion, "high_watermark", floor)
			s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
				GameID:  req.GameId,
				Env:     req.Env,
				AgentID: req.AgentId,
				Code:    reg.WarningCodeSDKVersionBehind,
				Message: msg,
			})
		case sdkversion.VerdictReject:
			rejectedByProc[p.ServiceId] = p.FunctionIds
			msg := fmt.Sprintf("service_id=%s sdk=%s/%s behind high watermark %s by 2+ minors or 1+ major: function registration rejected (%d functions)", p.ServiceId, language, p.SdkVersion, floor, len(p.FunctionIds))
			*warningTexts = append(*warningTexts, msg)
			s.logger.Warn("sdk version floor rejected registration", "agent_id", req.AgentId, "service_id", p.ServiceId, "sdk_language", language, "sdk_version", p.SdkVersion, "high_watermark", floor, "functions", len(p.FunctionIds))
			s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
				GameID:  req.GameId,
				Env:     req.Env,
				AgentID: req.AgentId,
				Code:    reg.WarningCodeSDKVersionFloorRejected,
				Message: msg,
			})
		}
	}
	for _, fids := range rejectedByProc {
		for _, fid := range fids {
			// 交叉提供保护：任一放行进程（或无 SDK 自报进程）也声明该函数
			// 时不剔——函数仍可由放行方提供，拒绝不应造成可用性缺口。
			if !allowedDeclared[fid] {
				outcome.rejected[fid] = true
			}
		}
	}
	return outcome
}

func (s *ControlService) handleRegisterCapabilitiesRequest(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	if s.upstream != nil {
		return s.upstream.HandleRegisterCapabilities(ctx, req)
	}

	if req.Provider == nil || req.Provider.Id == "" {
		return nil, fmt.Errorf("provider metadata is required")
	}

	manifestData, err := decompressManifest(req.ManifestJsonGz)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest (gzip): %w", err)
	}

	providerCaps := reg.OpenAPIProviderCaps{
		ID:         req.Provider.Id,
		Version:    req.Provider.Version,
		Lang:       req.Provider.Lang,
		SDK:        req.Provider.Sdk,
		OpenAPIDoc: manifestData,
		UpdatedAt:  time.Now(),
	}
	// Upsert 仅在 ID 为空时报错；ID 已由上游 provider 注册校验保证非空。
	_ = s.registry.UpsertOpenAPIProvider(providerCaps)

	s.logger.Info("Provider capabilities registered", "provider_id", req.Provider.Id)

	return &agentv1.RegisterCapabilitiesResponse{}, nil
}

func validateAndNormalizeFunctions(items []*agentv1.FunctionDescriptor) ([]*agentv1.FunctionDescriptor, []registerWarning) {
	if len(items) == 0 {
		return nil, nil
	}
	byID := make(map[string]*agentv1.FunctionDescriptor, len(items))
	warnings := make([]registerWarning, 0)
	for idx, f := range items {
		if f == nil {
			warnings = append(warnings, registerWarning{Code: "nil_function", Message: fmt.Sprintf("functions[%d] is nil and skipped", idx)})
			continue
		}
		fid := strings.TrimSpace(strings.ToLower(f.GetId()))
		if fid == "" {
			warnings = append(warnings, registerWarning{Code: "empty_function_id", Message: fmt.Sprintf("functions[%d] has empty function_id and skipped", idx)})
			continue
		}
		if !functionIDPattern.MatchString(fid) {
			warnings = append(warnings, registerWarning{Code: "invalid_function_id", FunctionID: fid, Version: f.GetVersion(), Message: fmt.Sprintf("function_id=%s invalid format (expected lowercase dotted id) and skipped", fid)})
			continue
		}
		version := strings.TrimSpace(f.GetVersion())
		if !isValidSemver(version) {
			warnings = append(warnings, registerWarning{Code: "invalid_version", FunctionID: fid, Version: version, Message: fmt.Sprintf("function_id=%s version=%s invalid semver and skipped", fid, version)})
			continue
		}
		if forbiddenKey, ok := descriptorPresentationField(f); ok {
			warnings = append(warnings, registerWarning{Code: "function_presentation_field_not_allowed", FunctionID: fid, Version: version, Message: fmt.Sprintf("function_id=%s registers presentation field %q; function registration only accepts executable capability contract and is skipped", fid, forbiddenKey)})
			continue
		}
		f.Id = fid
		f.Resource = strings.TrimSpace(strings.ToLower(f.GetResource()))
		f.Operation = strings.TrimSpace(strings.ToLower(f.GetOperation()))
		if prev, ok := byID[fid]; ok {
			compare := compareSemver(f.GetVersion(), prev.GetVersion())
			if compare > 0 {
				warnings = append(warnings, registerWarning{Code: "duplicate_function_id", FunctionID: fid, Version: f.GetVersion(), Message: fmt.Sprintf("duplicate function_id=%s detected; keep higher version %s over %s", fid, f.GetVersion(), prev.GetVersion())})
				byID[fid] = f
			} else {
				warnings = append(warnings, registerWarning{Code: "duplicate_function_id", FunctionID: fid, Version: prev.GetVersion(), Message: fmt.Sprintf("duplicate function_id=%s detected; keep higher version %s over %s", fid, prev.GetVersion(), f.GetVersion())})
			}
			continue
		}
		byID[fid] = f
	}
	out := make([]*agentv1.FunctionDescriptor, 0, len(byID))
	for _, f := range byID {
		out = append(out, f)
	}
	return out, warnings
}

func descriptorPresentationField(f *agentv1.FunctionDescriptor) (string, bool) {
	if f == nil {
		return "", false
	}
	// FunctionDescriptor has no presentation fields. Reject unknown protobuf
	// fields at the registration boundary instead of silently carrying an old
	// page/UI extension into FunctionContract.
	if unknown := f.ProtoReflect().GetUnknown(); len(unknown) > 0 {
		return "unknown_proto_fields", true
	}
	// Schemas belong to the executable capability contract, but they must not
	// carry dashboard presentation extensions (x-menu, x-table-columns,
	// formily, ...). Reject the whole descriptor instead of silently
	// persisting a smuggled UI hint.
	if field, _, ok := registrationguard.ScanJSON(f.GetInputSchema()); ok {
		return "inputSchema." + field, true
	}
	if field, _, ok := registrationguard.ScanJSON(f.GetOutputSchema()); ok {
		return "outputSchema." + field, true
	}
	return "", false
}

func isValidSemver(v string) bool {
	return versionutil.IsValid(v)
}

func compareSemver(a, b string) int {
	parse := func(raw string) [3]int {
		s := strings.TrimPrefix(strings.TrimSpace(raw), "v")
		parts := strings.SplitN(s, "-", 2)
		num := parts[0]
		p := strings.Split(num, ".")
		out := [3]int{0, 0, 0}
		for i := 0; i < len(p) && i < 3; i++ {
			n, err := strconv.Atoi(p[i])
			if err != nil {
				return out
			}
			out[i] = n
		}
		return out
	}
	va := parse(a)
	vb := parse(b)
	for i := 0; i < 3; i++ {
		if va[i] > vb[i] {
			return 1
		}
		if va[i] < vb[i] {
			return -1
		}
	}
	return 0
}

func decompressManifest(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("manifest data is empty")
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

// ControlHandler wraps ControlService to implement the Handler interface.
type ControlHandler struct {
	service *ControlService
}

// NewControlHandler creates a new control handler.
func NewControlHandler(service *ControlService) *ControlHandler {
	return &ControlHandler{service: service}
}

func (h *ControlHandler) HandleRegister(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	return h.service.handleRegisterRequest(ctx, req, "")
}

func (h *ControlHandler) HandleHeartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	return h.service.handleHeartbeatRequest(ctx, req)
}

func (h *ControlHandler) HandleRegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	return h.service.handleRegisterCapabilitiesRequest(ctx, req)
}
