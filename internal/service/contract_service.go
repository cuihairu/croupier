package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// PageProposalGeneratorVersion identifies the page proposal generator
// revision. It participates in proposalComparableDigest, so bumping it makes
// every stored proposal compare as changed on the next rebuild pass and get
// regenerated from the current generator (e.g. after label-fallback changes).
// page-generator:3：humanize 兜底上线（裸 key → 可读标题），存量提案
// 在下一轮 RebuildAllProposals 时按版本差异自动重生成。
const PageProposalGeneratorVersion = "page-generator:3"

// ContractService manages FunctionContract persistence and semantic rebuilding.
type ContractService struct {
	db               *gorm.DB
	contractModel    *model.FunctionContractModel
	capabilityModel  *model.ResourceCapabilityModel
	semanticsModel   *model.CapabilitySemanticsModel
	versionModel     *model.CapabilitySemanticVersionModel
	contractVersions *model.FunctionContractVersionModel
	proposalModel    *model.PageProposalModel
	proposalVersions *model.PageProposalVersionModel
	blockedIssues    *model.BlockedProposalIssueModel
	auditSvc         *audit.AuditService
	alertModel       *model.AlertModel
}

// WithAlertModel 注入告警模型（可选）：schema 破坏性变更写告警中心。
func (s *ContractService) WithAlertModel(m *model.AlertModel) *ContractService {
	s.alertModel = m
	return s
}

// WithAuditService 注入审计服务（可选）：契约更新时写 function.contract_updated
// 审计事件（含 schema 兼容性 diff 摘要）。
func (s *ContractService) WithAuditService(auditSvc *audit.AuditService) *ContractService {
	s.auditSvc = auditSvc
	return s
}

// NewContractService creates the service.
func NewContractService(db *gorm.DB) *ContractService {
	return &ContractService{
		db:               db,
		contractModel:    model.NewFunctionContractModel(db),
		capabilityModel:  model.NewResourceCapabilityModel(db),
		semanticsModel:   model.NewCapabilitySemanticsModel(db),
		versionModel:     model.NewCapabilitySemanticVersionModel(db),
		contractVersions: model.NewFunctionContractVersionModel(db),
		proposalModel:    model.NewPageProposalModel(db),
		proposalVersions: model.NewPageProposalVersionModel(db),
		blockedIssues:    model.NewBlockedProposalIssueModel(db),
		alertModel:       model.NewAlertModel(db),
	}
}

// RebuildContractFromFunctionMeta rebuilds a FunctionContract from the
// explicit registration contract. This is called when a function is
// registered or updated.
func (s *ContractService) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, input spec.FunctionContractInput) error {
	return s.rebuildContract(ctx, gameID, env, source, input, spec.ExecutionStateBound)
}

// CreateUnboundContract 为上传管线生成 unbound 契约（D1/D4、T4）：仅当同
// (game_id, env, function_id) 契约不存在时创建——已有 bound 契约的 operation
// 不降级，重复上传幂等（仅不存在时建）。落库与注册路径共用 rebuildContract
// （归一/digest/诊断一致），执行状态为 unbound；模板联动由上传管线在事务
// 提交后单次收口（T5 RegenerateContractTemplates）。
func (s *ContractService) CreateUnboundContract(ctx context.Context, gameID, env, source string, input spec.FunctionContractInput) (bool, error) {
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	input.ID = strings.TrimSpace(input.ID)
	if input.ID == "" {
		return false, fmt.Errorf("functionId is required for unbound contract")
	}
	existing, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, input.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, fmt.Errorf("find existing contract %s: %w", input.ID, err)
	}
	if existing != nil {
		return false, nil
	}
	if err := s.rebuildContract(ctx, gameID, env, source, input, spec.ExecutionStateUnbound); err != nil {
		return false, err
	}
	return true, nil
}

// rebuildContract 是注册（bound）与上传管线（unbound）共用的契约落库路径：
// 归一 → digest → schema diff → upsert → 审计/告警。组件模板联动（T2）由
// 各事务边界在提交后经 RegenerateContractTemplates 单次收口（见函数尾注）。
func (s *ContractService) rebuildContract(ctx context.Context, gameID, env, source string, input spec.FunctionContractInput, executionState spec.ExecutionState) error {
	// 入库前归一（scope 字段）：投影层（contract_projection）读取时对
	// gameID/env/function_id TrimSpace，写入侧在此同步归一，保证两侧
	// 对称——避免上游传入（或 DB 直写/迁移数据）的首尾空白让发布链
	// FindByScopeAndFunctionID 精确匹配落空。函数入口先归一，使本函数
	// 内所有查找/审计/落库使用同一组 scope 值。
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	input.ID = strings.TrimSpace(input.ID)

	if violation, ok := registrationguard.FindPresentationViolation(nil, input.InputSchema, input.OutputSchema); ok {
		return fmt.Errorf("function contract contains forbidden presentation field %q at %s", violation.Field, violation.Location)
	}

	// D3/T6 自动绑定：bound 注册路径（运行时/SDK/显式 provider 绑定）命中
	// 同 (game_id, env, function_id) 的 unbound 契约时，先回补运行时元数据
	// 缺失的分类字段再按 bound 落库（OpenAPI 上传是富分类来源，SDK 注册常
	// 只带 schema——与 functionMetaInputForBinding 的「OpenAPI 优先、运行时
	// 兜底」同一取舍）。查询失败不阻断注册（跳过自动绑定与 schema diff，
	// 与既有容错一致）。模板/提案 freshness 走既有 digest 门控：回补/运行时
	// schema 变化即重算；内容完全一致时无谓重算被跳过（心跳风暴防护）。
	existing, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, input.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Warn("auto-bind: failed to load existing contract",
			"game_id", gameID, "env", env, "function_id", input.ID, "error", err)
		existing = nil
	}
	autoBound := existing != nil &&
		executionState == spec.ExecutionStateBound &&
		spec.NormalizeExecutionState(existing.ExecutionState) == spec.ExecutionStateUnbound
	if autoBound {
		backfillInputFromClassification(&input, existing)
	}

	// 1. Normalize the descriptor
	normInput := normalizer.DescriptorInput{
		ID:                input.ID,
		Version:           input.Version,
		Summary:           input.Summary,
		Description:       input.Description,
		InputSchema:       input.InputSchema,
		OutputSchema:      input.OutputSchema,
		Resource:          input.Resource,
		Operation:         input.Operation,
		Capability:        input.Capability,
		Execution:         input.Execution,
		ApprovalRequired:  input.ApprovalRequired,
		ApprovalPolicyKey: input.ApprovalPolicyKey,
		Risk:              input.Risk,
		Permission:        input.Permission,
		Enabled:           input.Enabled,
		Tags:              input.Tags,
		TimeoutMs:         input.TimeoutMs,
	}
	result := normalizer.Normalize(normInput)
	if validationErr := contractValidationError(result.Diagnostics); validationErr != nil {
		return validationErr
	}

	// 2. Compute source digest from the canonical normalized contract.
	digest := computeDigest(result.Function)

	// 3. Build FunctionContract
	// 入库前归一，保证与投影层 TrimSpace 对称（normalizer 已对多数
	// 字段 trim，此处显式兜底 Version 等 normalizer 不处理的字段，
	// 不依赖上游实现细节）。
	contract := &model.FunctionContract{
		GameID:       gameID,
		Env:          env,
		FunctionID:   strings.TrimSpace(result.Function.ID),
		Version:      strings.TrimSpace(result.Function.Version),
		Enabled:      result.Function.Enabled,
		Deprecated:   input.Deprecated,
		ResourceKey:  strings.TrimSpace(result.Function.Resource),
		OperationKey: strings.TrimSpace(result.Function.Operation),
		Capability:   mustParseCapability(string(result.Function.Capability)),
		Execution:    string(result.Function.Execution),
		// D2/T3：执行状态由调用路径决定——运行时/SDK 注册（bound）或
		// 上传管线 unbound 物料（T4）；运行时命中同 function_id 的
		// unbound 行由自动绑定（T6）翻转。
		ExecutionState: string(executionState),
		TimeoutMs:      timeoutMsToInt32(result.Function.TimeoutMs),
		Approval:       approvalPolicyToJSONMap(result.Function.Approval),
		Risk:           mustParseRisk(string(result.Function.Risk)),
		Permission:     strings.TrimSpace(result.Function.Permission),
		InputSchema:    normalizeSchemaToJSON(json.RawMessage(result.Function.InputSchema)),
		OutputSchema:   normalizeSchemaToJSON(json.RawMessage(result.Function.OutputSchema)),
		Summary:        toJSONMap(result.Function.Summary),
		Description:    toJSONMap(result.Function.Description),
		Tags:           toJSON(result.Function.Tags),
		Source:         source,
		SourceDigest:   digest,
		Diagnostics:    toJSON(result.Diagnostics),
	}

	// 4. Schema 兼容性 diff（F12）：与库中现有契约对比，破坏性变更
	// 写入 Diagnostics 告警（不阻断注册，由告警/可视化层消费）。
	// existing 由函数入口统一加载（T6 自动绑定检测复用同一查询）。
	diagnostics, diffFindings, isUpdate := mergeSchemaDiffDiagnostics(existing, contract, toJSON(result.Diagnostics))
	// ExecutionState 空值归一与 UpsertContract 内部一致——B2 历史判据
	// （contractContentChanged）必须在归一后比较，否则「跳过写」与
	// 「跳过历史」判据漂移。
	if strings.TrimSpace(contract.ExecutionState) == "" {
		contract.ExecutionState = string(spec.ExecutionStateBound)
	}
	// B2：版本历史判据与 UpsertContract 的跳过写完全一致。
	contractChanged := contractContentChanged(existing, contract)
	// 契约更新时把上一版 schema 存入 prev 列——sync-selectors 的 rename
	// 精确推断依赖它；schema 未变的重注册在 UpsertContract 内被跳过，
	// prev 不会被无意义刷新。
	if existing != nil {
		contract.PrevInputSchema = existing.InputSchema
		contract.PrevOutputSchema = existing.OutputSchema
	}

	// 5. Upsert contract
	contract.Diagnostics = diagnostics
	if err := s.contractModel.UpsertContract(ctx, contract); err != nil {
		return fmt.Errorf("upsert function contract: %w", err)
	}
	// 摘除宽限：宽限期内重注册清除 pending 标记——瞬态摘除（重启窗口/
	// 门槛误配/闪断）不产生 removed+created 版本噪音，契约行全程存活。
	// 无 pending 时是零成本 no-op；清除失败降级告警（下轮清扫最多误删
	// 一次后由本轮注册物化恢复，不阻断注册）。
	if err := s.contractModel.ClearRemovalPending(ctx, gameID, env, input.ID); err != nil {
		slog.Warn("clear contract removal pending failed",
			"game_id", gameID, "env", env, "function_id", input.ID, "error", err)
	}

	// B2：内容变化才落版本历史。历史是衍生审计数据（与 proposals/templates
	// 重建同档），写失败降级为告警不阻断注册——迁移 0029 + 启动期
	// MinimumRequiredVersion=29 已保证表存在，此处软失败是为并发/边界容错，
	// 绝不在注册事务内因软删竞态等把整条注册带崩。
	if contractChanged {
		if err := s.appendContractVersion(ctx, existing, contract, diffFindings); err != nil {
			slog.Warn("append function contract version failed",
				"game_id", gameID, "env", env, "function_id", input.ID, "error", err)
		}
	}

	// F13：契约更新审计（function.contract_updated，含 diff 摘要）。
	s.logContractUpdateAudit(ctx, gameID, env, source, input.ID, isUpdate, diffFindings)

	// F：schema 破坏性变更写告警中心（主动提醒运营；按内容去重防骚扰）。
	s.warnBreakingSchemaChanges(ctx, gameID, env, source, input.ID, diffFindings)

	slog.Info("rebuilt function contract",
		"game_id", gameID,
		"env", env,
		"function_id", input.ID,
		"resource", input.Resource,
		"capability", input.Capability)

	// T2/D1 模板联动不在此处内联触发：组件模板表经进程级注入闭包走全局
	// 连接，而本函数常在调用方事务内执行（agent 注册 / OpenAPI 绑定），
	// 文件型 sqlite 下跨连接写模板表会与事务写锁互等待 busy_timeout(60s)
	// 自死锁（E2E probe register 超时根因）。收口统一挪到各事务边界提交
	// 后单次执行（RegenerateContractTemplates）：注册 → registry Store；
	// 上传/更新源 → finishUploadPipeline；显式绑定/解绑 → openapi service。

	// D3/T6：unbound→bound 翻转落库后写自动绑定审计事件（失败不阻塞）。
	if autoBound {
		s.logAutoBindingAudit(ctx, gameID, env, source, input.ID, existing)
	}

	return nil
}

// ListContracts lists all contracts in a scope.
func (s *ContractService) ListContracts(ctx context.Context, gameID, env string) ([]*model.FunctionContract, error) {
	return s.contractModel.ListByScope(ctx, gameID, env)
}

// GetContract gets a contract by function ID.
func (s *ContractService) GetContract(ctx context.Context, gameID, env, functionID string) (*model.FunctionContract, error) {
	return s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
}

// RemoveFunctionContract removes an executable registration contract and its
// standalone generated proposal. Published snapshots are intentionally kept:
// freshness then reports binding_function_missing and rejects execution.
func (s *ContractService) RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (string, error) {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return "", nil
	}
	contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find function contract %s: %w", functionID, err)
	}
	resourceKey := strings.TrimSpace(contract.ResourceKey)
	if err := s.contractModel.DeleteByScopeAndFunctionID(ctx, gameID, env, functionID); err != nil {
		return "", fmt.Errorf("delete function contract %s: %w", functionID, err)
	}
	// B2：删除也是可查询的历史事件（快照保留删除前最后一版）。
	// 同 rebuildContract：历史写失败降级告警，不回滚已成功的删除。
	if err := s.appendRemovedContractVersion(ctx, contract); err != nil {
		slog.Warn("append function contract version failed",
			"game_id", gameID, "env", env, "function_id", functionID, "error", err)
	}
	if err := s.removeStandaloneProposalsForFunction(ctx, gameID, env, functionID); err != nil {
		return "", err
	}
	if err := s.resolveBlockedIssue(ctx, gameID, env, resourceKey, functionID); err != nil {
		return "", err
	}
	return resourceKey, nil
}

// MarkContractRemovalPending 注册链函数消失时打摘除宽限标记（不立即
// 真删）。契约行不存在时静默返回（无对象可标记，语义等同已删）。
func (s *ContractService) MarkContractRemovalPending(ctx context.Context, gameID, env, functionID string) error {
	if s.contractModel == nil {
		return nil
	}
	marked, err := s.contractModel.MarkRemovalPending(ctx, gameID, env, functionID)
	if err != nil {
		return fmt.Errorf("mark function contract %s removal pending: %w", functionID, err)
	}
	if marked {
		slog.Info("function contract removal pending",
			"game_id", gameID, "env", env, "function_id", functionID)
	}
	return nil
}

// removalSweepBatchSize 单轮清扫最多认领的候选行数：候选查询按
// removal_pending_at 最旧优先 LIMIT 截断，剩余候选 30s 后下一轮继续，
// 积压再大也不拖垮单轮预算（索引 idx_function_contracts_removal_pending
// 支持过滤+排序）。
const removalSweepBatchSize = 200

// FinalizeExpiredContractRemovals 清扫宽限已到期的摘除 pending 行：
// 条件删除原子认领（HA 多实例清扫只有一个命中，removed 历史不双写；
// 宽限内重注册已清 pending，条件删除命中不了，函数不会被误删）。
// 真删后补 removed 版本历史 + 提案清理 + 资源能力/页面提案重建 +
// 组件模板收口（与 RemoveFunctionContract 即时路径同净效果）。衍生
// 动作失败不中断整轮清扫（其余候选继续），但聚合进返回错误——否则
// 契约已真删而衍生状态残缺时运维不可见、下轮也不再处理。
func (s *ContractService) FinalizeExpiredContractRemovals(ctx context.Context, grace time.Duration) (int, error) {
	if s.contractModel == nil {
		return 0, nil
	}
	candidates, err := s.contractModel.ListExpiredPendingRemovals(ctx, time.Now().Add(-grace), removalSweepBatchSize)
	if err != nil {
		return 0, fmt.Errorf("list expired pending removals: %w", err)
	}
	finalized := 0
	var errs []error
	templatesToRegen := [][2]string{} // 去重后的 (gameID, env) 收口清单
	for _, contract := range candidates {
		deleted, err := s.contractModel.DeleteIfRemovalPending(ctx, contract.GameID, contract.Env, contract.FunctionID)
		if err != nil {
			// 认领失败不提前收整轮：其余候选继续清扫，失败聚合返回
			//（与衍生动作失败同一口径，R6/R7）。
			errs = append(errs, fmt.Errorf("claim pending removal %s: %w", contract.FunctionID, err))
			continue
		}
		if !deleted {
			continue // 被另一实例认领，或宽限内已重注册
		}
		if err := s.appendRemovedContractVersion(ctx, contract); err != nil {
			slog.Warn("append function contract version failed",
				"game_id", contract.GameID, "env", contract.Env, "function_id", contract.FunctionID, "error", err)
			errs = append(errs, fmt.Errorf("function %s removed history: %w", contract.FunctionID, err))
		}
		if err := s.removeStandaloneProposalsForFunction(ctx, contract.GameID, contract.Env, contract.FunctionID); err != nil {
			slog.Warn("remove standalone proposals failed",
				"game_id", contract.GameID, "env", contract.Env, "function_id", contract.FunctionID, "error", err)
			errs = append(errs, fmt.Errorf("function %s standalone proposals: %w", contract.FunctionID, err))
		}
		if err := s.resolveBlockedIssue(ctx, contract.GameID, contract.Env, strings.TrimSpace(contract.ResourceKey), contract.FunctionID); err != nil {
			slog.Warn("resolve blocked issue failed",
				"game_id", contract.GameID, "env", contract.Env, "function_id", contract.FunctionID, "error", err)
			errs = append(errs, fmt.Errorf("function %s blocked issue: %w", contract.FunctionID, err))
		}
		if resourceKey := strings.TrimSpace(contract.ResourceKey); resourceKey != "" {
			if err := s.RebuildResourceCapability(ctx, contract.GameID, contract.Env, resourceKey); err != nil {
				slog.Warn("rebuild resource capability after pending removal failed",
					"game_id", contract.GameID, "env", contract.Env, "resource", resourceKey, "error", err)
				errs = append(errs, fmt.Errorf("resource %s capability: %w", resourceKey, err))
			}
			// 与即时移除路径同净效果：资源仍有其他存活契约时重生成
			// 资源页提案（摘除已删函数的 binding，旧提案若直接发布会
			// 产出 binding_function_missing 的必败绑定）；资源已无存活
			// 契约（能力聚合被清）时，资源页提案一并摘除。
			_, capErr := s.capabilityModel.FindByScopeAndResourceKey(ctx, contract.GameID, contract.Env, resourceKey)
			switch {
			case errors.Is(capErr, gorm.ErrRecordNotFound):
				if err := s.removeResourceProposal(ctx, contract.GameID, contract.Env, resourceKey); err != nil {
					slog.Warn("remove resource proposal after pending removal failed",
						"game_id", contract.GameID, "env", contract.Env, "resource", resourceKey, "error", err)
					errs = append(errs, fmt.Errorf("resource %s proposal removal: %w", resourceKey, err))
				}
			case capErr != nil:
				slog.Warn("find resource capability after pending removal failed",
					"game_id", contract.GameID, "env", contract.Env, "resource", resourceKey, "error", capErr)
				errs = append(errs, fmt.Errorf("resource %s capability lookup: %w", resourceKey, capErr))
			default:
				if err := s.RebuildProposalsForResource(ctx, contract.GameID, contract.Env, resourceKey); err != nil {
					slog.Warn("rebuild resource proposals after pending removal failed",
						"game_id", contract.GameID, "env", contract.Env, "resource", resourceKey, "error", err)
					errs = append(errs, fmt.Errorf("resource %s proposal rebuild: %w", resourceKey, err))
				}
			}
		}
		finalized++
		scopeSeen := false
		for _, scope := range templatesToRegen {
			if scope[0] == contract.GameID && scope[1] == contract.Env {
				scopeSeen = true
				break
			}
		}
		if !scopeSeen {
			templatesToRegen = append(templatesToRegen, [2]string{contract.GameID, contract.Env})
		}
	}
	// 组件模板收口：只对发生真删的 scope 重建一次（模板内容含契约
	// 形态，残留死函数模板与提案同理误导运营）。
	for _, scope := range templatesToRegen {
		if err := s.RegenerateContractTemplates(ctx, scope[0], scope[1]); err != nil {
			slog.Warn("regenerate contract templates after pending removal failed",
				"game_id", scope[0], "env", scope[1], "error", err)
			errs = append(errs, fmt.Errorf("templates %s/%s: %w", scope[0], scope[1], err))
		}
	}
	return finalized, errors.Join(errs...)
}

// ListResourceCapabilities lists all resource capabilities in a scope.
func (s *ContractService) ListResourceCapabilities(ctx context.Context, gameID, env string) ([]*model.ResourceCapability, error) {
	return s.capabilityModel.ListByScope(ctx, gameID, env)
}
