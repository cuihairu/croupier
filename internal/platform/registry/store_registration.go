package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/cuihairu/croupier/internal/platform/registry/sdkversion"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// contractRemovalGracePeriod 是摘除宽限默认时长：注册面函数消失后契约先标
// pending，宽限期内重注册自动清除，超期才由 SweepExpiredContractRemovals
// 真删。包级变量沿用 backgroundLoopInterval 的测试注入惯例（无配置先例，
// 不加配置项）。
var contractRemovalGracePeriod = 10 * time.Minute

// SweepExpiredContractRemovals 清扫摘除宽限到期的 pending 契约（后台循环
// 每 backgroundLoopInterval 调一次）。未接线 contractService（内存注册面）
// 时为 no-op。
func (s *Store) SweepExpiredContractRemovals(ctx context.Context) (int, error) {
	if s == nil || s.contractService == nil {
		return 0, nil
	}
	return s.contractService.FinalizeExpiredContractRemovals(ctx, contractRemovalGracePeriod)
}

// carryProviderSessionHistory 在注册合并（内存态）时保留 provider 的进程内
// 历史观测（#27②）：FirstSeenUnix 取两侧更早的非零观测（新观测缺省补
// now）；VersionHWM 走高不回退，不可解析版本永不入选（对齐 sdkversion 高
// 水位语义，全不可解析时保持空、展示层回退 Version）。prev 为 nil 表示
// 全新会话（首次注册）。next 会被原地修改后整体替换会话的 Providers。
func carryProviderSessionHistory(prev, next []ProviderSession) {
	prevByID := make(map[string]ProviderSession, len(prev))
	for _, p := range prev {
		prevByID[p.ProviderID] = p
	}
	now := time.Now().Unix()
	for i := range next {
		n := &next[i]
		first := n.FirstSeenUnix
		if p, ok := prevByID[n.ProviderID]; ok && p.FirstSeenUnix > 0 && (first == 0 || p.FirstSeenUnix < first) {
			first = p.FirstSeenUnix
		}
		if first == 0 {
			first = now
		}
		n.FirstSeenUnix = first
		n.VersionHWM = providerVersionHWM(prevByID[n.ProviderID], *n)
	}
}

// providerVersionHWM 合并两次观测的版本高水位：候选为两侧 HWM 与当次
// Version，取可解析者中最高；无一个可解析时返回空（展示层回退 Version）。
func providerVersionHWM(prev, cur ProviderSession) string {
	best := ""
	for _, cand := range []string{prev.VersionHWM, prev.Version, cur.VersionHWM, cur.Version} {
		if !sdkversion.Parseable(cand) {
			continue
		}
		if best == "" || sdkversion.Higher(cand, best) {
			best = cand
		}
	}
	return best
}

// regenTemplatesAfterRegistration 在注册写入（事务或直写）成功后单次收口
// 组件模板重建（T2）。快照无 Added/Changed/Removed 时不触发——心跳/重连
// 风暴下函数集未变即不空转（对齐原 digest 门控语义）。失败仅告警（写
// registrationWarnings，Code=template_regen_failed，UI 可见），不影响
// 已提交的注册结果（手动 regenerate 仍是兜底）。
func (s *Store) regenTemplatesAfterRegistration(ctx context.Context, session *AgentSession, diff functionSnapshotDiff) {
	if s.contractService == nil || session == nil {
		return
	}
	if len(diff.Added) == 0 && len(diff.Changed) == 0 && len(diff.Removed) == 0 {
		return
	}
	if err := s.contractService.RegenerateContractTemplates(ctx, session.GameID, session.Env); err != nil {
		slog.Default().Warn("auto regenerate component templates after agent registration failed (manual regenerate remains as fallback)",
			"agent_id", session.AgentID,
			"game_id", session.GameID,
			"env", session.Env,
			"error", err)
		s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
			GameID:  session.GameID,
			Env:     session.Env,
			AgentID: session.AgentID,
			Code:    WarningCodeTemplateRegenFailed,
			Message: err.Error(),
		})
	}
}

// registrationRebuildPlan 注册事务内收集的衍生重建计划（提交后执行）。
// 快照无变化（心跳/重连）时集合为空，提交后重建自然跳过。
type registrationRebuildPlan struct {
	Resources           []string
	StandaloneFunctions []string
}

func (s *Store) materializeScopedTransaction(
	scopeCtx context.Context,
	session *AgentSession,
	materialize func(context.Context, *AgentSession, functionSnapshotDiff) (*registrationRebuildPlan, error),
	diff functionSnapshotDiff,
) (*registrationRebuildPlan, error) {
	scopeDB := dbctx.Resolve(scopeCtx, s.db)
	if scopeDB == nil {
		return nil, fmt.Errorf("registration projection database is not initialized")
	}
	var plan *registrationRebuildPlan
	if err := scopeDB.Transaction(func(tx *gorm.DB) error {
		p, err := materialize(dbctx.WithDB(scopeCtx, tx), session, diff)
		if err != nil {
			return err
		}
		plan = p
		return nil
	}); err != nil {
		return nil, err
	}
	return plan, nil
}

// materializeAgent 物化注册的权威状态（契约 + 资源能力聚合），并收集
// 页面提案的衍生重建计划。提案重建不在本事务内执行——见
// rebuildProposalsAfterRegistration。
func (s *Store) materializeAgent(ctx context.Context, session *AgentSession, sessionDiff functionSnapshotDiff) (*registrationRebuildPlan, error) {
	if s.contractService == nil || session == nil || session.Functions == nil {
		return nil, nil
	}
	var rebuildErrors []error
	resources := stringSet(sessionDiff.Resources)
	standaloneFunctions := make(map[string]bool)
	for _, functionID := range sortedFunctionIDs(session.Functions) {
		meta := session.Functions[functionID]
		if err := s.rebuildFunctionContract(ctx, session.GameID, session.Env, functionID, meta, resources, standaloneFunctions); err != nil {
			rebuildErrors = append(rebuildErrors, err)
		}
	}
	for _, functionID := range sessionDiff.Removed {
		if meta, ok := s.survivingFunctionMeta(session.AgentID, session.GameID, session.Env, functionID); ok {
			if err := s.rebuildFunctionContract(ctx, session.GameID, session.Env, functionID, meta, resources, standaloneFunctions); err != nil {
				rebuildErrors = append(rebuildErrors, err)
			}
			continue
		}
		// 摘除宽限：函数从本次注册中消失时只打 pending 标记，不立即真删
		// ——重启窗口/门槛误配/闪断的瞬态摘除不再制造 removed+created
		// 版本噪音与页面 stale 抖动；宽限期（默认 10min）后由
		// SweepExpiredContractRemovals 真删并补 removed 历史。资源能力
		// 聚合此刻无需重建（契约行仍存活，聚合内容不变）。
		if err := s.contractService.MarkContractRemovalPending(ctx, session.GameID, session.Env, functionID); err != nil {
			rebuildErrors = append(rebuildErrors, fmt.Errorf("mark function contract removal pending %s: %w", functionID, err))
			continue
		}
	}
	for _, resource := range sortedStringSet(resources) {
		if err := s.contractService.RebuildResourceCapability(ctx, session.GameID, session.Env, resource); err != nil {
			rebuildErrors = append(rebuildErrors, fmt.Errorf("rebuild resource capability %s: %w", resource, err))
			continue
		}
	}
	if len(rebuildErrors) > 0 {
		return nil, fmt.Errorf("agent registration contract rebuild failed: %w", errors.Join(rebuildErrors...))
	}
	return &registrationRebuildPlan{
		Resources:           sortedStringSet(resources),
		StandaloneFunctions: sortedStringSet(standaloneFunctions),
	}, nil
}

// rebuildProposalsAfterRegistration 在注册写入（事务或直写）成功后重建
// 页面提案（衍生数据，可由已提交契约全量重算）。失败仅告警
// （Code=proposal_rebuild_failed，UI 可见），不影响已提交的注册结果——
// 手动 POST /pages/proposals/rebuild 仍是兜底。
func (s *Store) rebuildProposalsAfterRegistration(ctx context.Context, session *AgentSession, plan *registrationRebuildPlan) {
	if s.contractService == nil || session == nil || plan == nil {
		return
	}
	for _, resource := range plan.Resources {
		if err := s.contractService.RebuildProposalsForResource(ctx, session.GameID, session.Env, resource); err != nil {
			slog.Default().Warn("rebuild page proposals after agent registration failed (manual proposals rebuild remains as fallback)",
				"agent_id", session.AgentID,
				"game_id", session.GameID,
				"env", session.Env,
				"resource", resource,
				"error", err)
			s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
				GameID:  session.GameID,
				Env:     session.Env,
				AgentID: session.AgentID,
				Code:    WarningCodeProposalRebuildFailed,
				Message: fmt.Sprintf("resource %s: %s", resource, err.Error()),
			})
		}
	}
	for _, functionID := range plan.StandaloneFunctions {
		if err := s.contractService.RebuildProposalForFunction(ctx, session.GameID, session.Env, functionID); err != nil {
			slog.Default().Warn("rebuild standalone page proposal after agent registration failed (manual proposals rebuild remains as fallback)",
				"agent_id", session.AgentID,
				"game_id", session.GameID,
				"env", session.Env,
				"function_id", functionID,
				"error", err)
			s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
				GameID:  session.GameID,
				Env:     session.Env,
				AgentID: session.AgentID,
				Code:    WarningCodeProposalRebuildFailed,
				Message: fmt.Sprintf("function %s: %s", functionID, err.Error()),
			})
		}
	}
}

func functionSnapshot(session *AgentSession) map[string]FunctionMeta {
	if session == nil || session.Functions == nil {
		return nil
	}
	functions := make(map[string]FunctionMeta, len(session.Functions))
	for functionID, meta := range session.Functions {
		functions[functionID] = meta
	}
	return functions
}

func (s *Store) prepareRegistrationOperation(target, previous *AgentSession) (*AgentRegistrationOperationDB, error) {
	targetJSON, err := json.Marshal(target)
	if err != nil {
		return nil, fmt.Errorf("marshal target registration session: %w", err)
	}
	previousJSON, err := json.Marshal(previous)
	if err != nil {
		return nil, fmt.Errorf("marshal previous registration session: %w", err)
	}
	operation := &AgentRegistrationOperationDB{
		OperationID:     uuid.NewString(),
		AgentID:         target.AgentID,
		GameID:          target.GameID,
		Env:             target.Env,
		PreviousSession: string(previousJSON),
		TargetSession:   string(targetJSON),
		Status:          "pending",
	}
	if err := s.db.Create(operation).Error; err != nil {
		return nil, fmt.Errorf("create registration recovery operation: %w", err)
	}
	return operation, nil
}

func (s *Store) markRegistrationOperation(operationID, status string, cause error) {
	if s == nil || s.db == nil || strings.TrimSpace(operationID) == "" {
		return
	}
	lastError := ""
	if cause != nil {
		lastError = cause.Error()
	}
	if err := s.markRegistrationOperationWithDB(s.db, operationID, status, lastError); err != nil {
		slog.Default().Error("failed to update registration recovery operation", "operation_id", operationID, "status", status, "error", err)
	}
}

func (s *Store) markRegistrationOperationWithDB(db *gorm.DB, operationID, status, lastError string) error {
	if db == nil {
		return fmt.Errorf("registration recovery database is not initialized")
	}
	return db.Model(&AgentRegistrationOperationDB{}).
		Where("operation_id = ?", operationID).
		Updates(map[string]interface{}{"status": status, "last_error": lastError}).Error
}

func (s *Store) rebuildFunctionContract(
	ctx context.Context,
	gameID string,
	env string,
	functionID string,
	meta FunctionMeta,
	resources map[string]bool,
	standaloneFunctions map[string]bool,
) error {
	input := spec.FunctionContractInput{
		ID:                functionID,
		Version:           meta.Version,
		Enabled:           meta.Enabled,
		Deprecated:        meta.Deprecated,
		Summary:           meta.Summary,
		Description:       meta.Description,
		InputSchema:       meta.InputSchema,
		OutputSchema:      meta.OutputSchema,
		Resource:          meta.Resource,
		Operation:         meta.Operation,
		Capability:        meta.Capability,
		Execution:         meta.Execution,
		ApprovalRequired:  meta.ApprovalRequired,
		ApprovalPolicyKey: meta.ApprovalPolicyKey,
		Risk:              meta.Risk,
		Permission:        meta.Permission,
		Tags:              meta.Tags,
	}
	if err := s.contractService.RebuildContractFromFunctionMeta(ctx, gameID, env, "sdk", input); err != nil {
		return fmt.Errorf("rebuild function contract %s: %w", functionID, err)
	}
	if meta.Resource != "" {
		resources[meta.Resource] = true
	} else {
		standaloneFunctions[functionID] = true
	}
	return nil
}

func (s *Store) previousFunctions(agentID string) map[string]FunctionMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.agents[agentID]
	if current == nil || current.Functions == nil {
		return nil
	}
	functions := make(map[string]FunctionMeta, len(current.Functions))
	for functionID, meta := range current.Functions {
		functions[functionID] = meta
	}
	return functions
}

func sortedFunctionIDs(functions map[string]FunctionMeta) []string {
	ids := make([]string, 0, len(functions))
	for functionID := range functions {
		ids = append(ids, functionID)
	}
	sort.Strings(ids)
	return ids
}

func classifyFunctionSnapshot(previous, current map[string]FunctionMeta) functionSnapshotDiff {
	// nil means the registration omitted the snapshot; it must not delete all
	// existing functions during heartbeat-compatible registrations.
	if current == nil {
		return functionSnapshotDiff{}
	}
	diff := functionSnapshotDiff{}
	resources := map[string]struct{}{}
	for functionID, previousMeta := range previous {
		currentMeta, ok := current[functionID]
		if !ok {
			diff.Removed = append(diff.Removed, functionID)
			if resource := strings.TrimSpace(previousMeta.Resource); resource != "" {
				resources[resource] = struct{}{}
			}
			continue
		}
		if !reflect.DeepEqual(previousMeta, currentMeta) {
			diff.Changed = append(diff.Changed, functionID)
		}
		for _, resource := range []string{previousMeta.Resource, currentMeta.Resource} {
			if resource = strings.TrimSpace(resource); resource != "" {
				resources[resource] = struct{}{}
			}
		}
	}
	for functionID, currentMeta := range current {
		if _, ok := previous[functionID]; !ok {
			diff.Added = append(diff.Added, functionID)
		}
		if resource := strings.TrimSpace(currentMeta.Resource); resource != "" {
			resources[resource] = struct{}{}
		}
	}
	sort.Strings(diff.Added)
	sort.Strings(diff.Changed)
	sort.Strings(diff.Removed)
	for resource := range resources {
		diff.Resources = append(diff.Resources, resource)
	}
	sort.Strings(diff.Resources)
	return diff
}

func sortedStringSet(values map[string]bool) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			items = append(items, value)
		}
	}
	sort.Strings(items)
	return items
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = true
		}
	}
	return set
}

// validateAgentFunctionContracts keeps the registry write boundary aligned
// with the SDK and OpenAPI adapters. A caller that bypasses the control
// handler must not persist a dashboard presentation extension in a schema.
func validateAgentFunctionContracts(functions map[string]FunctionMeta) error {
	functionIDs := make([]string, 0, len(functions))
	for functionID := range functions {
		functionIDs = append(functionIDs, functionID)
	}
	sort.Strings(functionIDs)

	for _, functionID := range functionIDs {
		meta := functions[functionID]
		if violation, ok := registrationguard.FindPresentationViolation(nil, meta.InputSchema, meta.OutputSchema); ok {
			return fmt.Errorf("function %q %s contains forbidden presentation field %q", functionID, violation.Location, violation.Field)
		}
	}
	return nil
}
