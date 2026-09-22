package service

// 本文件补 contract_history.go / contract_service.go 中尚未覆盖的可达分支：
// B2 版本历史写路径的守卫与序列化失败、摘除宽限清扫（FinalizeExpiredContract
// Removals）真删后各衍生动作的错误聚合分支、模板收口去重等。只新增测试，
// 不改产品代码。错误注入全部确定性：gorm 回调（Before 阶段按表名/Dest
// 形态注错）与 sqlite 触发器（RAISE ABORT / RAISE IGNORE），无时序依赖。

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupCoverageEDB 文件型 sqlite + 全量模型（含 function_contract_versions）：
// 触发器类用例要求连接池共享同一物理库（":memory:" 每连接独立库会随机失效）；
// 摘除历史的 AppendVersion 依赖版本表存在——成功路径不能被静默降级掩盖。
func setupCoverageEDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/cov_e.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.FunctionContract{},
		&model.FunctionContractVersion{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
		&model.PageSpec{},
		&model.PublishedPageSpec{},
		&model.PageVersion{},
		&model.TermDictionary{},
		&model.Alert{},
	))
	return db
}

// covEValidSchema 合法入参 schema（快照可正常序列化）。
const covEValidSchema = `{"type":"object","properties":{"id":{"type":"string"}}}`

// covESeedPendingContract 直插一行已过宽限的 pending 契约：绕开注册链，
// 精确控制 removal_pending_at（固定过去时刻，不依赖 MarkRemovalPending 与
// 清扫调用之间的时钟推进）与 schema 形态；清扫链只消费行状态。
func covESeedPendingContract(t *testing.T, db *gorm.DB, gameID, env, functionID, resourceKey string, pendingAge time.Duration, inputSchema string) {
	t.Helper()
	pending := time.Now().Add(-pendingAge)
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID:           gameID,
		Env:              env,
		FunctionID:       functionID,
		ResourceKey:      resourceKey,
		InputSchema:      model.JSON(inputSchema),
		RemovalPendingAt: &pending,
	}).Error)
}

// covEInjectQueryFail 只让指定表的 SELECT 失败（Query/Row 链），写放行：
// 用于区分「前置读成功、后置读失败」的分支（全表注错会让前置查询先炸）。
func covEInjectQueryFail(db *gorm.DB, table string) (remove func()) {
	name := "test.cov_e.failquery." + table
	fail := func(tx *gorm.DB) {
		if matchTable(tx, table) {
			_ = tx.AddError(errors.New("injected query failure for " + table))
		}
	}
	_ = db.Callback().Query().Before("gorm:query").Register(name, fail)
	_ = db.Callback().Row().Before("gorm:row").Register(name, fail)
	return func() {
		_ = db.Callback().Query().Remove(name)
		_ = db.Callback().Row().Remove(name)
	}
}

// ---------------------------------------------------------------------------
// contract_history.go
// ---------------------------------------------------------------------------

// contractContentChanged：既有行为软删（deleted_at 非零）时视同内容变化——
// 重注册语义是复活，必须写历史（与 UpsertContract 的 Unscoped 命中软删后
// 必写保持同一判据）。
func TestCoverageE_ContractContentChangedSoftDeletedExisting(t *testing.T) {
	existing := &model.FunctionContract{Model: gorm.Model{
		DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
	}}
	assert.True(t, contractContentChanged(existing, &model.FunctionContract{}))
	assert.True(t, contractContentChanged(nil, &model.FunctionContract{}), "无既有行恒为变化")
}

// appendContractVersion：版本模型未注入（裁剪构造）时 no-op——历史是衍生
// 审计数据，缺表/缺模型不阻断注册主流程。
func TestCoverageE_AppendContractVersionNilModelGuard(t *testing.T) {
	s := &ContractService{}
	require.NoError(t, s.appendContractVersion(context.Background(), nil, &model.FunctionContract{FunctionID: "fn"}, nil))
	require.NoError(t, s.appendContractVersion(context.Background(), nil, nil, nil))
}

// appendContractVersion：快照序列化失败必须把错误交回调用方（调用方降级为
// 告警）。可达构造：InputSchema 列写入非法 JSON 文本——normalizeJSONSchema
// 原样透传、spec.JSONSchema.MarshalJSON 原样返回，encoding/json 的 compact
// 校验失败，json.Marshal 报错。
func TestCoverageE_AppendContractVersionMarshalFailure(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	err := svc.appendContractVersion(context.Background(), nil, &model.FunctionContract{
		GameID: "g-e", Env: "e-e", FunctionID: "fn.bad-schema",
		InputSchema: model.JSON(`{"type":"object"`), // 截断的非法 JSON
	}, nil)
	require.Error(t, err)
}

// appendRemovedContractVersion：nil 守卫 + 同款序列化失败分支。
func TestCoverageE_AppendRemovedContractVersionBranches(t *testing.T) {
	s := &ContractService{}
	require.NoError(t, s.appendRemovedContractVersion(context.Background(), &model.FunctionContract{FunctionID: "fn"}))

	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	err := svc.appendRemovedContractVersion(context.Background(), &model.FunctionContract{
		GameID: "g-e", Env: "e-e", FunctionID: "fn.bad-schema",
		OutputSchema: model.JSON(`{"type":`), // 非法 JSON
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// contract_service.go：注册 / 回填 / 审计
// ---------------------------------------------------------------------------

// CreateUnboundContract：同 (game_id, env, function_id) 已有契约时幂等返回
// (false, nil)——已有 bound 契约的 operation 不因重复上传降级。
func TestCoverageE_CreateUnboundContractIdempotentWhenExists(t *testing.T) {
	db := setupCoverageEDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	created, err := svc.CreateUnboundContract(ctx, "g-e", "e-e", "openapi", unboundMaterialInput("player.get"))
	require.NoError(t, err)
	assert.True(t, created, "首传新建 unbound 契约")

	created, err = svc.CreateUnboundContract(ctx, "g-e", "e-e", "openapi", unboundMaterialInput("player.get"))
	require.NoError(t, err)
	assert.False(t, created, "重复上传幂等：已存在即不重建")
}

// rebuildContract：executionState 传空（遗留调用形态）时归一为 bound 落库
// ——与 UpsertContract 内部归一对齐，否则零值行在语义比较里恒不等，
// 「内容无变化跳过写」失效。
func TestCoverageE_RebuildContractBlankExecutionStateNormalized(t *testing.T) {
	db := setupCoverageEDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	require.NoError(t, svc.rebuildContract(ctx, "g-e", "e-e", "sdk", sdkRegistrationInput("fn.blank-state"), spec.ExecutionState("")))

	stored, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "g-e", "e-e", "fn.blank-state")
	require.NoError(t, err)
	assert.Equal(t, string(spec.ExecutionStateBound), stored.ExecutionState)
}

// rebuildContract：宽限标记清除失败只降级告警、不阻断注册（下轮清扫最多
// 误删一次后由本轮注册物化恢复）。注错按 Dest 形态精确命中——
// ClearRemovalPending 的 UpdateColumn 语句 Dest 是含 removal_pending_at 键的
// map，而 UpsertContract 的 Save 语句 Dest 是结构体，互不影响。
func TestCoverageE_RebuildContractClearRemovalPendingFailureIsDegraded(t *testing.T) {
	db := setupCoverageEDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-e", "e-e", "sdk", sdkRegistrationInput("fn.grace")))
	require.NoError(t, svc.MarkContractRemovalPending(ctx, "g-e", "e-e", "fn.grace"))

	name := "test.cov_e.fail_clear_pending"
	_ = db.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
		if !matchTable(tx, "function_contracts") {
			return
		}
		if dest, ok := tx.Statement.Dest.(map[string]interface{}); ok {
			if _, hit := dest["removal_pending_at"]; hit {
				_ = tx.AddError(errors.New("injected clear removal pending failure"))
			}
		}
	})
	defer func() { _ = db.Callback().Update().Remove(name) }()

	second := sdkRegistrationInput("fn.grace")
	second.Version = "2.0.0" // 内容变化强制 Save，绕开「无变化跳过写」
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-e", "e-e", "sdk", second),
		"清除 pending 失败不得把注册带崩")

	// 历史链不受清除失败影响：created + updated 两条。
	var versions int64
	require.NoError(t, db.Model(&model.FunctionContractVersion{}).
		Where("game_id = ? AND env = ? AND function_id = ?", "g-e", "e-e", "fn.grace").
		Count(&versions).Error)
	assert.Equal(t, int64(2), versions)
}

// backfillInputFromClassification：nil 入参守卫（防御式直调不 panic）。
func TestCoverageE_BackfillInputFromClassificationNilGuards(t *testing.T) {
	backfillInputFromClassification(nil, &model.FunctionContract{ResourceKey: "player"})
	input := &spec.FunctionContractInput{}
	backfillInputFromClassification(input, nil)
	assert.Empty(t, input.Resource)
}

// covEFailingAuditStore 审计写入恒失败：驱动 logAutoBindingAudit 的
// auditSvc.Log 错误分支（写审计失败只告警，不影响注册主流程）。
type covEFailingAuditStore struct {
	*audit.InMemoryAuditStore
}

func (s *covEFailingAuditStore) Create(*audit.AuditRecord) error {
	return errors.New("cov-e audit create failure")
}

func TestCoverageE_LogAutoBindingAuditFailureIsDegraded(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db).WithAuditService(
		audit.NewAuditService(&covEFailingAuditStore{audit.NewInMemoryAuditStore()}, nil))
	// previous 非 nil / nil 两种形态各触发一次，均只告警不上抛。
	svc.logAutoBindingAudit(context.Background(), "g-e", "e-e", "sdk", "fn.x", &model.FunctionContract{Source: "openapi"})
	svc.logAutoBindingAudit(context.Background(), "g-e", "e-e", "sdk", "fn.x", nil)
}

// MarkContractRemovalPending：契约模型未注入（裁剪构造）静默返回；打标失败
// 包装错误上抛（清扫链与注册链调用方依赖该错误决定是否重试）。
func TestCoverageE_MarkContractRemovalPendingBranches(t *testing.T) {
	require.NoError(t, (&ContractService{}).MarkContractRemovalPending(context.Background(), "g", "e", "fn"))

	db := setupCoverageEDB(t)
	ctx := context.Background()
	svc := NewContractService(db)
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-e", "e-e", "sdk", sdkRegistrationInput("fn.mark")))

	remove := injectWriteFailCallback(db, "function_contracts")
	defer remove()
	err := svc.MarkContractRemovalPending(ctx, "g-e", "e-e", "fn.mark")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mark function contract")
}

// ---------------------------------------------------------------------------
// FinalizeExpiredContractRemovals：真删后衍生动作的错误聚合分支
// ---------------------------------------------------------------------------

// 模型未注入（裁剪构造）：整轮清扫短路返回 (0, nil)。
func TestCoverageE_FinalizeNilContractModelGuard(t *testing.T) {
	finalized, err := (&ContractService{}).FinalizeExpiredContractRemovals(context.Background(), time.Minute)
	require.NoError(t, err)
	assert.Zero(t, finalized)
}

// 候选列举失败：整轮提前返回包装错误。
func TestCoverageE_FinalizeListExpiredPendingRemovalsFailure(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.list-fail", "", time.Hour, covEValidSchema)

	remove := injectFailCallback(db, "function_contracts")
	defer remove()

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list expired pending removals")
	assert.Zero(t, finalized)
}

// 认领失败（DeleteIfRemovalPending 报错）聚合进返回错误且其余候选继续的
// 控制流成立（本例单候选即终止）。契约删除是软删（UPDATE deleted_at），
// 用 BEFORE UPDATE 触发器按 NEW.deleted_at 非空精确拦下这次「删除」。
func TestCoverageE_FinalizeClaimPendingRemovalFailureAggregated(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.claim", "", time.Hour, covEValidSchema)

	require.NoError(t, db.Exec(`CREATE TRIGGER cov_e_abort_soft_delete BEFORE UPDATE ON function_contracts
		WHEN NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL
		BEGIN SELECT RAISE(ABORT, 'soft delete blocked'); END`).Error)

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claim pending removal")
	assert.Zero(t, finalized)
	// 行未被本实例认领：仍可查到且 pending 标记仍在。
	contract, findErr := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(context.Background(), "g-e", "e-e", "fn.claim")
	require.NoError(t, findErr)
	require.NotNil(t, contract.RemovalPendingAt)
}

// DeleteIfRemovalPending 命中 0 行（HA 下被另一实例先认领、或宽限内已重
// 注册）：不报错、不计 finalized、不触发衍生动作。RAISE(IGNORE) 让软删
// UPDATE 跳过该行——语句成功且 changes=0，等价于条件删除未命中。
func TestCoverageE_FinalizeClaimLostToConcurrentInstanceSkips(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.lost", "", time.Hour, covEValidSchema)

	require.NoError(t, db.Exec(`CREATE TRIGGER cov_e_ignore_soft_delete BEFORE UPDATE ON function_contracts
		WHEN NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL
		BEGIN SELECT RAISE(IGNORE); END`).Error)

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.NoError(t, err)
	assert.Zero(t, finalized, "认领未命中不计 finalized、不产生错误")
	_, findErr := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(context.Background(), "g-e", "e-e", "fn.lost")
	require.NoError(t, findErr, "行仍在（未被本实例删除）")
}

// removed 版本历史写失败：真删不回退（finalized 仍计 1），错误聚合返回。
// 可达构造同 TestCoverageE_AppendContractVersionMarshalFailure——schema 列
// 为非法 JSON 文本，快照序列化失败。
func TestCoverageE_FinalizeRemovedHistoryFailureAggregated(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.bad-schema", "", time.Hour, `{"type":"object"`)

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "removed history")
	assert.Equal(t, 1, finalized, "衍生动作失败不回退真删计数")
}

// standalone 提案摘除失败：聚合返回。page_proposals 的 DeleteByScopeAndKey
// 是 Unscoped 硬删——预置目标行后用 BEFORE DELETE 触发器拦（0 行命中的
// DELETE 不会触发行级触发器，所以必须先落行）。
func TestCoverageE_FinalizeStandaloneProposalRemovalFailureAggregated(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.standalone", "", time.Hour, covEValidSchema)
	require.NoError(t, db.Create(&model.PageProposal{
		GameID: "g-e", Env: "e-e", ProposalKey: "operation:fn.standalone", PageKey: "p-standalone",
	}).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER cov_e_abort_proposal_delete BEFORE DELETE ON page_proposals
		BEGIN SELECT RAISE(ABORT, 'proposal delete blocked'); END`).Error)

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "standalone proposals")
	assert.Equal(t, 1, finalized)
}

// 阻断议题解决失败：聚合返回。Resolve 是 Updates(map)（Update 回调链），
// 表级写注错即可命中（0 行命中的 UPDATE 语句同样执行回调）。
func TestCoverageE_FinalizeResolveBlockedIssueFailureAggregated(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.issue", "", time.Hour, covEValidSchema)

	remove := injectWriteFailCallback(db, "blocked_proposal_issues")
	defer remove()

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked issue")
	assert.Equal(t, 1, finalized)
}

// 资源能力重建失败：聚合返回。最后一个契约被清扫后 RebuildResourceCapability
// 走 removeResourceDerivedState——能力行删除是软删（UPDATE deleted_at），用
// BEFORE UPDATE 触发器精确拦这次删除；行级触发器需要命中行，故预置能力行。
func TestCoverageE_FinalizeRebuildCapabilityFailureAggregated(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.capfail", "player", time.Hour, covEValidSchema)
	require.NoError(t, db.Create(&model.ResourceCapability{
		GameID: "g-e", Env: "e-e", ResourceKey: "player",
	}).Error)

	require.NoError(t, db.Exec(`CREATE TRIGGER cov_e_abort_cap_delete BEFORE UPDATE ON resource_capabilities
		WHEN NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL
		BEGIN SELECT RAISE(ABORT, 'capability soft delete blocked'); END`).Error)

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource player capability")
	assert.Equal(t, 1, finalized)
}

// 真删后能力查询 NotFound → 资源页提案摘除失败：聚合返回。能力行已被上一步
// RebuildResourceCapability 软删（First 自动过滤软删行 → NotFound），随后
// removeResourceProposal 的硬删被 BEFORE DELETE 触发器拦（预置 resource:player
// 行使命中非 0）。
func TestCoverageE_FinalizeResourceProposalRemovalFailureAggregated(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.last", "player", time.Hour, covEValidSchema)
	require.NoError(t, db.Create(&model.PageProposal{
		GameID: "g-e", Env: "e-e", ProposalKey: "resource:player", PageKey: "p-player",
	}).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER cov_e_abort_resource_proposal_delete BEFORE DELETE ON page_proposals
		BEGIN SELECT RAISE(ABORT, 'resource proposal delete blocked'); END`).Error)

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource player proposal removal")
	assert.Equal(t, 1, finalized)
}

// 真删后能力查询报错（非 NotFound）：聚合返回。前面 RebuildResourceCapability
// 在 0 存活契约路径只做删除不读能力表——SELECT 注错精确命中清扫尾部的
// capability 查询而不影响前置链路。
func TestCoverageE_FinalizeCapabilityLookupFailureAggregated(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.qfail", "player", time.Hour, covEValidSchema)

	remove := covEInjectQueryFail(db, "resource_capabilities")
	defer remove()

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource player capability lookup")
	assert.Equal(t, 1, finalized)
}

// 资源仍有存活契约（能力查询命中 default 分支）→ 资源页提案重建失败：
// 聚合返回。page_proposals 写注错只拦 UpsertProposal 的 Save（Update 链），
// 前置的 standalone 硬删走 Delete 链不受影响。
func TestCoverageE_FinalizeResourceProposalRebuildFailureAggregated(t *testing.T) {
	db := setupCoverageEDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	// 幸存函数是 collection_query（资源页提案的存在前提），被摘函数是 action。
	register := func(id, capability string) {
		require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-e", "e-e", "sdk", FunctionMetaInput{
			ID:           id,
			Version:      "1.0.0",
			Enabled:      true,
			InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}},"total":{"type":"integer"}}}`,
			Resource:     "player",
			Operation:    "op",
			Capability:   capability,
			Execution:    "sync",
		}))
	}
	register("player.ban", "action")
	register("player.list", "collection_query")
	require.NoError(t, svc.RebuildResourceCapability(ctx, "g-e", "e-e", "player"))
	require.NoError(t, svc.RebuildProposalsForResource(ctx, "g-e", "e-e", "player"))

	// 直接把宽限起点拨到一小时前：不依赖 Mark 与清扫之间的时钟推进。
	past := time.Now().Add(-time.Hour)
	require.NoError(t, db.Model(&model.FunctionContract{}).
		Where("game_id = ? AND env = ? AND function_id = ?", "g-e", "e-e", "player.ban").
		UpdateColumn("removal_pending_at", past).Error)

	remove := injectWriteFailCallback(db, "page_proposals")
	defer remove()

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource player proposal rebuild")
	assert.Equal(t, 1, finalized)
}

// 同 scope 两个候选都真删：模板收口清单必须去重——RegenerateContractTemplates
// 对同 (game_id, env) 只调用一次。断言稳定性质（调用次数），与候选遍历顺序
// 无关。
func TestCoverageE_FinalizeTemplateRegenScopeDeduplicated(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.a", "", 2*time.Hour, covEValidSchema)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.b", "", time.Hour, covEValidSchema)

	var calls int32
	SetContractTemplateRegenerator(func(context.Context, string, string) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	defer SetContractTemplateRegenerator(nil)

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, 2, finalized)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "同 scope 模板收口去重为一次")
}

// 模板收口失败：聚合返回（模板内容含契约形态，残留死函数模板误导运营，
// 失败必须对运维可见）。经包级注入点 SetContractTemplateRegenerator 注入
// 恒败重建器（仓库既定的测试注入手法）。
func TestCoverageE_FinalizeTemplateRegenFailureAggregated(t *testing.T) {
	db := setupCoverageEDB(t)
	svc := NewContractService(db)
	covESeedPendingContract(t, db, "g-e", "e-e", "fn.tpl", "", time.Hour, covEValidSchema)

	SetContractTemplateRegenerator(func(context.Context, string, string) error {
		return errors.New("cov-e template regen boom")
	})
	defer SetContractTemplateRegenerator(nil)

	finalized, err := svc.FinalizeExpiredContractRemovals(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "templates g-e/e-e")
	assert.Equal(t, 1, finalized, "模板失败不回退真删计数")
}
