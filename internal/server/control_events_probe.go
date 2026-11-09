package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/core/report"
	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// probePolicyConfigKey config_versions 里的探测采样策略键（namespace=ops；
// 面板与 agent 两侧同键，评审简档 §13.3 C）。策略对 server 不透明面有限：
// 发布预检与 agent apply 共用 core/healthprobe.ParseProbePolicy 校验。
const probePolicyConfigKey = "probe.policy"

// handleProbePolicyConfig 响应 agent 的采样策略下行拉取（capture gates 同款
// 通道语义）：从注册会话取 scope，读 config_versions 最新版本回给 agent。
// 无记录/读失败/分库解析失败统一回空版本（agent 语义 = 保持现状，降级为
// 最后已知策略/本地配置间隔）——拉取是周期轮询，单轮读异常不应放大为通道
// 错误打断节奏。
func (s *ControlService) handleProbePolicyConfig(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.ProbePolicyConfigRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProbePolicyConfigRequest: %w", err)
	}

	s.mu.RLock()
	configModel := s.configModel
	gameDBs := s.gameDBs
	s.mu.RUnlock()
	if configModel == nil {
		return nil, fmt.Errorf("config model not configured")
	}

	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	s.registry.Mu().RLock()
	sess := s.registry.AgentsUnsafe()[agentID]
	s.registry.Mu().RUnlock()
	if sess == nil {
		return nil, fmt.Errorf("agent session not found: %s", agentID)
	}
	gameID, env := strings.TrimSpace(sess.GameID), strings.TrimSpace(sess.Env)
	if gameID == "" || env == "" {
		return nil, fmt.Errorf("agent %s missing game scope", agentID)
	}

	dbCtx := ctx
	if gameDBs != nil {
		resolved, _, err := gameDBs.Resolve(ctx, gameID, env)
		if err != nil {
			s.logger.Warn("probe policy: resolve game db failed", "agent", agentID,
				"gameId", gameID, "env", env, "error", err)
			return proto.Marshal(&opsv1.ProbePolicyConfigResponse{})
		}
		dbCtx = resolved
	}

	rec, err := configModel.FindLatestByScope(dbCtx, probePolicyConfigKey, gameID, env)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("probe policy: read config_versions failed", "agent", agentID,
				"gameId", gameID, "env", env, "error", err)
		}
		return proto.Marshal(&opsv1.ProbePolicyConfigResponse{})
	}
	return proto.Marshal(&opsv1.ProbePolicyConfigResponse{
		Version:    strconv.Itoa(rec.Version),
		PolicyJson: rec.Value,
	})
}

// syntheticsTargetsConfigKey config_versions 里的 synthetics 目标键
// （namespace=ops；与 agent 本地 synthetics.targets[] 同形状，面板发布侧
// internal/api/ops 与 agent 拉取侧 agents/synthetics/runner 同键）。
const syntheticsTargetsConfigKey = "synthetics.targets"

// handleSyntheticsTargetsConfig 响应 agent 的 synthetics 目标下行拉取
// （capture gates/probe policy/devops allowlist 同款通道语义）：从注册会话
// 取 scope，读 config_versions 最新版本回给 agent。无记录/读失败/分库解析
// 失败统一回空版本（agent 语义 = 保持现状，keep last-good）——拉取是周期
// 轮询，单轮读异常不应放大为通道错误打断节奏。
func (s *ControlService) handleSyntheticsTargetsConfig(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.SyntheticsTargetsConfigRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal SyntheticsTargetsConfigRequest: %w", err)
	}

	s.mu.RLock()
	configModel := s.configModel
	gameDBs := s.gameDBs
	s.mu.RUnlock()
	if configModel == nil {
		return nil, fmt.Errorf("config model not configured")
	}

	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	s.registry.Mu().RLock()
	sess := s.registry.AgentsUnsafe()[agentID]
	s.registry.Mu().RUnlock()
	if sess == nil {
		return nil, fmt.Errorf("agent session not found: %s", agentID)
	}
	gameID, env := strings.TrimSpace(sess.GameID), strings.TrimSpace(sess.Env)
	if gameID == "" || env == "" {
		return nil, fmt.Errorf("agent %s missing game scope", agentID)
	}

	dbCtx := ctx
	if gameDBs != nil {
		resolved, _, err := gameDBs.Resolve(ctx, gameID, env)
		if err != nil {
			s.logger.Warn("synthetics targets: resolve game db failed", "agent", agentID,
				"gameId", gameID, "env", env, "error", err)
			return proto.Marshal(&opsv1.SyntheticsTargetsConfigResponse{})
		}
		dbCtx = resolved
	}

	rec, err := configModel.FindLatestByScope(dbCtx, syntheticsTargetsConfigKey, gameID, env)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("synthetics targets: read config_versions failed", "agent", agentID,
				"gameId", gameID, "env", env, "error", err)
		}
		return proto.Marshal(&opsv1.SyntheticsTargetsConfigResponse{})
	}
	return proto.Marshal(&opsv1.SyntheticsTargetsConfigResponse{
		Version:     strconv.Itoa(rec.Version),
		TargetsJson: rec.Value,
	})
}

// dbProbeTargetPrefix 归属派生 db 目标的目标名前缀：与本地 probe.targets[]
// 声明的目标名空间隔离，避免 ApplyTargets 撞名被整体拒绝。
const dbProbeTargetPrefix = "db:"

// devopsAllowlistConfigKey config_versions 里的 devops 会话白名单键
// （namespace=ops；面板发布侧 internal/api/ops 与 agent 拉取侧
// agents/devops/runner 同键，两侧键一致是 wire 契约）。
const devopsAllowlistConfigKey = "devops.allowlist"

// handleDevopsAllowlistConfig 响应 agent 的 devops 会话白名单下行拉取
// （capture gates/probe policy 同款通道语义）：从注册会话取 scope，读
// config_versions 最新版本回给 agent。无记录/读失败/分库解析失败统一回空
// 版本（agent 语义 = 保持现状，keep last-good）——拉取是周期轮询，单轮读
// 异常不应放大为通道错误打断节奏。
func (s *ControlService) handleDevopsAllowlistConfig(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.DevopsAllowlistConfigRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal DevopsAllowlistConfigRequest: %w", err)
	}

	s.mu.RLock()
	configModel := s.configModel
	gameDBs := s.gameDBs
	s.mu.RUnlock()
	if configModel == nil {
		return nil, fmt.Errorf("config model not configured")
	}

	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	s.registry.Mu().RLock()
	sess := s.registry.AgentsUnsafe()[agentID]
	s.registry.Mu().RUnlock()
	if sess == nil {
		return nil, fmt.Errorf("agent session not found: %s", agentID)
	}
	gameID, env := strings.TrimSpace(sess.GameID), strings.TrimSpace(sess.Env)
	if gameID == "" || env == "" {
		return nil, fmt.Errorf("agent %s missing game scope", agentID)
	}

	dbCtx := ctx
	if gameDBs != nil {
		resolved, _, err := gameDBs.Resolve(ctx, gameID, env)
		if err != nil {
			s.logger.Warn("devops allowlist: resolve game db failed", "agent", agentID,
				"gameId", gameID, "env", env, "error", err)
			return proto.Marshal(&opsv1.DevopsAllowlistConfigResponse{})
		}
		dbCtx = resolved
	}

	rec, err := configModel.FindLatestByScope(dbCtx, devopsAllowlistConfigKey, gameID, env)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("devops allowlist: read config_versions failed", "agent", agentID,
				"gameId", gameID, "env", env, "error", err)
		}
		return proto.Marshal(&opsv1.DevopsAllowlistConfigResponse{})
	}
	return proto.Marshal(&opsv1.DevopsAllowlistConfigResponse{
		Version:       strconv.Itoa(rec.Version),
		AllowlistJson: rec.Value,
	})
}

// dbProbeDriverName 把 DB 源的逻辑 driver 映射为 sql.Open 可用的 driver 名
// （dbmon 同款映射：postgres → pgx 标准库驱动；agent 二进制 blank import
// 注册 mysql/pgx）。
func dbProbeDriverName(driver string) string {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "postgres" {
		return "pgx"
	}
	return driver
}

// deriveProbeDBTargets 把归属 agent 的启用 DB 源派生成 db 探针目标声明
// （R4：拉取时派生——不写 config_versions、不新增表、无隐藏自动发布器，
// 拉取响应即真值）。version 为目标集内容 hash：目标集不变则 hash 不变，
// agent 侧 configsync 跳过无变化轮次；空集也有 hash——空集语义是「该
// agent 不再有归属源」，必须让 agent 卸掉动态探针。源按 id 升序保证 hash
// 稳定；重名源以 "#<id>" 后缀消歧（首用名不加），保证目标名唯一。
func deriveProbeDBTargets(items []model.DBSource) ([]healthprobe.TargetConfig, string) {
	sorted := append([]model.DBSource(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	targets := make([]healthprobe.TargetConfig, 0, len(sorted))
	seen := make(map[string]int, len(sorted))
	for _, src := range sorted {
		base := dbProbeTargetPrefix + strings.TrimSpace(src.Name)
		name := base
		if n := seen[base]; n > 0 {
			name = fmt.Sprintf("%s#%d", base, src.ID)
		}
		seen[base]++
		targets = append(targets, healthprobe.TargetConfig{
			Name:     name,
			Category: healthprobe.TargetCategoryGame,
			Type:     healthprobe.TargetTypeDB,
			DSN:      src.DSN,
			Driver:   dbProbeDriverName(src.Driver),
			// 连通判定硬超时对齐 DBChecker 默认（只读不写）；Query 留空 =
			// 只读断言 SELECT 1。
			Timeout: healthprobe.DefaultDBTimeout,
		})
	}
	body, err := json.Marshal(targets)
	if err != nil {
		// TargetConfig 只含标量字段，marshal 不会失败；防御性兜底空集 hash。
		body = []byte("[]")
	}
	sum := sha256.Sum256(body)
	return targets, hex.EncodeToString(sum[:])
}

// handleProbeDBTargets 响应 agent 的 db 探针目标下行拉取（R4：周期 db 探测
// 下放归属 agent 执行）。按 owner_agent_id = 调用方、(game_id, env) 与
// Agent 注册 scope 一致且 enabled 派生目标集；owner 指派不能替代归属
// 检查，跨 scope 源不静默下发。读失败回错误（agent 语义 = 保持现有目标，
// 下轮重试），派生成功一律带非空版本（空集也有 hash）。DSN 明文只在
// control conn 回给归属 agent（与 agent 本地
// probe.targets 配置同信任边界），本 handler 不落任何含 DSN 的日志。
func (s *ControlService) handleProbeDBTargets(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.ProbeDBTargetsConfigRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProbeDBTargetsConfigRequest: %w", err)
	}

	s.mu.RLock()
	dbModel := s.dbSourceModel
	s.mu.RUnlock()
	if dbModel == nil {
		return nil, fmt.Errorf("db source model not configured")
	}

	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	s.registry.Mu().RLock()
	sess := s.registry.AgentsUnsafe()[agentID]
	s.registry.Mu().RUnlock()
	if sess == nil {
		return nil, fmt.Errorf("agent session not found: %s", agentID)
	}

	items, err := dbModel.ListByOwnerAndScope(ctx, agentID, sess.GameID, sess.Env)
	if err != nil {
		s.logger.Warn("probe db targets: read db_sources failed", "agent", agentID, "error", err)
		return nil, fmt.Errorf("read db sources: %w", err)
	}
	targets, version := deriveProbeDBTargets(items)
	body, err := json.Marshal(targets)
	if err != nil {
		return nil, fmt.Errorf("marshal derived db targets: %w", err)
	}
	return proto.Marshal(&opsv1.ProbeDBTargetsConfigResponse{
		Version:     version,
		TargetsJson: string(body),
	})
}

// handleProbeEvent 接收 agent 探测/告警流事件（MsgAlertEvent 流事件，review
// §11 已决 2）。接收面：校验 session 身份 → 持久化与诊断环留证 → 窗口事件
// 消费为 Alert 生命周期（§12 R3；lifecycle 未装配时仍落库留环）→ 按 kind 记日志。
//
// kind 未知或 body 与 kind 不匹配一律拒收：流事件无响应通道可回退，错误只
// 能体现在日志与丢弃计数上，宁可少收不可错收（错收会污染诊断面）。
func (s *ControlService) handleProbeEvent(ctx context.Context, data []byte) ([]byte, error) {
	ev := &opsv1.AlertEvent{}
	if err := proto.Unmarshal(data, ev); err != nil {
		return nil, fmt.Errorf("unmarshal AlertEvent: %w", err)
	}

	agentID := strings.TrimSpace(ev.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	kind := strings.TrimSpace(ev.GetKind())
	switch kind {
	case "probe.window":
		if ev.GetWindow() == nil {
			return nil, fmt.Errorf("probe.window requires window payload")
		}
	case "probe.detail":
		if ev.GetDetail() == nil {
			return nil, fmt.Errorf("probe.detail requires detail payload")
		}
	default:
		return nil, fmt.Errorf("unsupported alert event kind %q", kind)
	}

	s.mu.RLock()
	sink := s.probeEvents
	probeModel := s.probeEventModel
	logger := s.logger
	s.mu.RUnlock()
	if probeModel != nil {
		if err := probeModel.Create(ctx, agentID, ev.GetGameId(), ev.GetEnv(), ev); err != nil {
			return nil, fmt.Errorf("persist probe event: %w", err)
		}
	}

	sink.add(agentID, ev)

	// R3：窗口事件消费为 Alert 生命周期（诊断环已留证先行；生命周期失败只
	// 记 warn 不影响接收——流事件无响应通道，接收成功即完成）。
	if kind == "probe.window" {
		s.mu.RLock()
		lifecycle := s.lifecycle
		s.mu.RUnlock()
		if lifecycle != nil {
			lifecycle.onWindow(ctx, agentID, ev)
		}
	}

	if logger != nil {
		switch kind {
		case "probe.window":
			w := ev.GetWindow()
			logger.Info("probe window event",
				"agent_id", agentID,
				"game_id", ev.GetGameId(),
				"env", ev.GetEnv(),
				"target", w.GetTarget(),
				"semantics", w.GetSemantics(),
				"window_kind", w.GetWindowKind(),
				"duration_ms", w.GetDurationMs(),
				"last_reason", w.GetLastReason(),
			)
		case "probe.detail":
			d := ev.GetDetail()
			logger.Info("probe failure detail",
				"agent_id", agentID,
				"game_id", ev.GetGameId(),
				"env", ev.GetEnv(),
				"target", d.GetTarget(),
				"semantics", d.GetSemantics(),
				"err", d.GetErr(),
				"latency_ms", d.GetLatencyMs(),
				"consecutive_failures", d.GetConsecutiveFailures(),
				"window_open", d.GetWindowOpen(),
				"details", d.GetDetails(),
			)
		}
	}
	return nil, nil
}

func (s *ControlService) normalizeProbeEvent(data []byte, identity *AgentIdentity) (*opsv1.AlertEvent, error) {
	ev := &opsv1.AlertEvent{}
	if err := proto.Unmarshal(data, ev); err != nil {
		return nil, fmt.Errorf("unmarshal AlertEvent: %w", err)
	}
	ev.AgentId = identity.AgentID
	ev.GameId = identity.GameID
	ev.Env = identity.Env
	return ev, nil
}

func (s *ControlService) handleProbeEventAsAgent(ctx context.Context, data []byte, identity *AgentIdentity) ([]byte, error) {
	ev, err := s.normalizeProbeEvent(data, identity)
	if err != nil {
		return nil, err
	}
	normalized, err := proto.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("marshal AlertEvent: %w", err)
	}
	return s.handleProbeEvent(ctx, normalized)
}

// ProbeEventSink 暴露诊断环只读访问（排查「这段时间服务不可用」用）。nil
// 接收者安全。
func (s *ControlService) ProbeEventSink() *probeEventSink {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.probeEvents
}

// ServerProbeAgentID server 自发探测的视角标注（§4.3）；真值在 core/report
// （上行桥组包侧），此别名供包内空载荷回退与测试引用。
const ServerProbeAgentID = report.ServerProbeAgentID

// HandleServerProbeEvent server 本机探针事件入站（R3 server 侧内核装配，
// §4.2/§4.3）：与 agent 上行同构——先诊断环留证，窗口事件再走 Alert 生命
// 周期（agentID 取事件载荷，空回 server）。lifecycle 未装配时仅留证。
func (s *ControlService) HandleServerProbeEvent(ctx context.Context, ev *opsv1.AlertEvent) {
	if s == nil || ev == nil {
		return
	}
	agentID := strings.TrimSpace(ev.GetAgentId())
	if agentID == "" {
		agentID = ServerProbeAgentID
	}
	s.mu.RLock()
	sink := s.probeEvents
	lifecycle := s.lifecycle
	logger := s.logger
	s.mu.RUnlock()

	sink.add(agentID, ev)
	if ev.GetKind() == "probe.window" && lifecycle != nil {
		lifecycle.onWindow(ctx, agentID, ev)
	}
	if logger != nil {
		logger.Info("server probe window event",
			"agent_id", agentID,
			"kind", ev.GetKind(),
			"target", ev.GetWindow().GetTarget(),
		)
	}
}

// SetProbeLifecycleModel 装配窗口→Alert 生命周期消费者（§12 R3）：sink 取
// 本服务自有诊断环，alertModel 为 nil 时不装配（仅诊断环留证）。
func (s *ControlService) SetProbeLifecycleModel(m *model.AlertModel) {
	if s == nil || m == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lifecycle = newProbeLifecycle(m, s.probeEvents, s.logger)
}
