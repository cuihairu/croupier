package registry_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 契约物化失败（权威状态写入失败）必须回滚整个注册：会话不可见、投影
// 全部还原。
func TestUpsertAgentRollsBackPersistentStateWhenMaterializationFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&registry.AgentSessionDB{},
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
	))

	store := registry.NewStoreWithDB(db)
	store.SetContractService(&contractFailingMaterializer{service: contractsvc.NewContractService(db)})
	err = store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]registry.FunctionMeta{
			"mail.send": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"message":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"ok":{"type":"boolean"}}}`,
			},
		},
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "injected contract failure")

	var sessions int64
	require.NoError(t, db.Model(&registry.AgentSessionDB{}).Where("agent_id = ?", "agent-1").Count(&sessions).Error)
	assert.Zero(t, sessions)
	_, err = model.NewFunctionContractModel(db).FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	store.Mu().RLock()
	assert.Nil(t, store.AgentsUnsafe()["agent-1"])
	store.Mu().RUnlock()
}

// 提案重建已移出注册事务：提交后失败降级为 proposal_rebuild_failed 告警，
// 注册结果（契约+能力+会话）不受影响；手动 proposals rebuild 是兜底。
func TestUpsertAgentSurvivesProposalRebuildFailureAfterCommit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&registry.AgentSessionDB{},
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
	))

	store := registry.NewStoreWithDB(db)
	store.SetContractService(&transactionalFailingMaterializer{service: contractsvc.NewContractService(db)})
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-prop",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]registry.FunctionMeta{
			"mail.send": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"message":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"ok":{"type":"boolean"}}}`,
			},
		},
	}))

	// 契约（权威状态）已提交可见。
	_, err = model.NewFunctionContractModel(db).FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	require.NoError(t, err)
	store.Mu().RLock()
	require.NotNil(t, store.AgentsUnsafe()["agent-prop"])
	store.Mu().RUnlock()

	// 提案（衍生数据）缺失，但失败已暴露为 UI 可见告警。
	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	warnings := store.ListRegistrationWarnings(registry.RegistrationWarningFilter{Code: registry.WarningCodeProposalRebuildFailed})
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "function mail.send")
	assert.Equal(t, "agent-prop", warnings[0].AgentID)

	// 兜底：手动重建提案（standalone 走 RebuildProposalForFunction）后提案可用。
	store.SetContractService(contractsvc.NewContractService(db))
	require.NoError(t, contractsvc.NewContractService(db).RebuildProposalForFunction(context.Background(), "demo-game", "development", "mail.send"))
	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	require.NoError(t, err)
}

func TestFailedAgentRegistrationRetryAndRestartAreConsistent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&registry.AgentSessionDB{},
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
	))

	session := &registry.AgentSession{
		AgentID:  "agent-retry",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Hour),
		LastSeen: time.Now(),
		Functions: map[string]registry.FunctionMeta{
			"mail.send": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"message":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"ok":{"type":"boolean"}}}`,
			},
		},
	}
	store := registry.NewStoreWithDB(db)
	store.SetContractService(&contractFailingMaterializer{service: contractsvc.NewContractService(db)})
	require.Error(t, store.UpsertAgent(session))

	store.SetContractService(contractsvc.NewContractService(db))
	require.NoError(t, store.UpsertAgent(session))
	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.Equal(t, "mail.send", contract.FunctionID)
	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	require.NoError(t, err)
	assert.Equal(t, "operation--mail.send", proposal.PageKey)

	restarted := registry.NewStoreWithDB(db)
	require.NoError(t, restarted.LoadFromDB(context.Background(), registry.NewAgentSessionModel(db)))
	restarted.Mu().RLock()
	recovered := restarted.AgentsUnsafe()["agent-retry"]
	restarted.Mu().RUnlock()
	require.NotNil(t, recovered)
	assert.Equal(t, session.Functions, recovered.Functions)
}

func TestUpsertAgentCompensatesGameProjectionWhenMetaSessionWriteFails(t *testing.T) {
	metaDB := openRegistrationTestDB(t)
	gameDB := openRegistrationTestDB(t)
	store := registry.NewStoreWithDB(metaDB)
	store.SetContractService(contractsvc.NewContractService(gameDB))
	store.SetScopeContextResolver(func(gameID, env string) context.Context {
		return dbctx.WithDB(context.Background(), gameDB)
	})

	// The game projection succeeds first. Make the meta session write fail by
	// removing only its session table; compensation must remove the projection.
	require.NoError(t, metaDB.Migrator().DropTable(&registry.AgentSessionDB{}))
	session := &registry.AgentSession{
		AgentID:  "agent-cross-db",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Hour),
		LastSeen: time.Now(),
		Functions: map[string]registry.FunctionMeta{
			"mail.send": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"message":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"ok":{"type":"boolean"}}}`,
			},
		},
	}
	err := store.UpsertAgent(session)
	require.Error(t, err)

	// 补偿走摘除宽限：反向 diff 对刚写入的契约打 pending 标记（不真删），
	// 契约与提案宽限期内保留，由清扫器最终收口。
	compensated, err := model.NewFunctionContractModel(gameDB).
		FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.NotNil(t, compensated.RemovalPendingAt, "补偿应打摘除宽限标记而非真删")
	_, err = model.NewPageProposalModel(gameDB).
		FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "提案重建在补偿发生时尚未执行（meta 先失败），本就不存在")

	var operation registry.AgentRegistrationOperationDB
	require.NoError(t, metaDB.Where("agent_id = ?", "agent-cross-db").First(&operation).Error)
	assert.Equal(t, "compensated", operation.Status)
	store.Mu().RLock()
	assert.Nil(t, store.AgentsUnsafe()["agent-cross-db"])
	store.Mu().RUnlock()
	// meta 提交失败时提交后重建尚未发生（未执行≠失败），不应产生告警。
	assert.Empty(t, store.ListRegistrationWarnings(registry.RegistrationWarningFilter{Code: registry.WarningCodeProposalRebuildFailed}))

	// Retrying the exact snapshot after the meta store recovers creates one
	// consistent session/projection pair, and a fresh process restores it.
	require.NoError(t, metaDB.AutoMigrate(&registry.AgentSessionDB{}))
	require.NoError(t, store.UpsertAgent(session))
	rebuilt, err := model.NewFunctionContractModel(gameDB).
		FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.Nil(t, rebuilt.RemovalPendingAt, "重试注册应清除补偿留下的 pending 标记")
	_, err = model.NewPageProposalModel(gameDB).
		FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	require.NoError(t, err)

	restarted := registry.NewStoreWithDB(metaDB)
	restarted.SetContractService(contractsvc.NewContractService(gameDB))
	restarted.SetScopeContextResolver(func(gameID, env string) context.Context {
		return dbctx.WithDB(context.Background(), gameDB)
	})
	require.NoError(t, restarted.LoadFromDB(context.Background(), registry.NewAgentSessionModel(metaDB)))
	restarted.Mu().RLock()
	recovered := restarted.AgentsUnsafe()[session.AgentID]
	restarted.Mu().RUnlock()
	require.NotNil(t, recovered)
	assert.Equal(t, session.Functions, recovered.Functions)
}

func TestLoadFromDBRecoversPendingCrossDatabaseRegistration(t *testing.T) {
	metaDB := openRegistrationTestDB(t)
	gameDB := openRegistrationTestDB(t)
	target := &registry.AgentSession{
		AgentID:  "agent-recover",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Hour),
		LastSeen: time.Now(),
		Functions: map[string]registry.FunctionMeta{
			"mail.send": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"message":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"ok":{"type":"boolean"}}}`,
			},
		},
	}
	contractService := contractsvc.NewContractService(gameDB)
	require.NoError(t, contractService.RebuildContractFromFunctionMeta(context.Background(), target.GameID, target.Env, "sdk", spec.FunctionContractInput{
		ID:           "mail.send",
		Version:      "1.0.0",
		Enabled:      true,
		InputSchema:  target.Functions["mail.send"].InputSchema,
		OutputSchema: target.Functions["mail.send"].OutputSchema,
	}))
	require.NoError(t, contractService.RebuildProposalForFunction(context.Background(), target.GameID, target.Env, "mail.send"))
	targetJSON, err := json.Marshal(target)
	require.NoError(t, err)
	require.NoError(t, metaDB.Create(&registry.AgentRegistrationOperationDB{
		OperationID:   "pending-recovery",
		AgentID:       target.AgentID,
		GameID:        target.GameID,
		Env:           target.Env,
		TargetSession: string(targetJSON),
		Status:        "pending",
	}).Error)

	restarted := registry.NewStoreWithDB(metaDB)
	restarted.SetContractService(contractService)
	restarted.SetScopeContextResolver(func(gameID, env string) context.Context {
		return dbctx.WithDB(context.Background(), gameDB)
	})
	require.NoError(t, restarted.LoadFromDB(context.Background(), registry.NewAgentSessionModel(metaDB)))

	// pending 操作回放走摘除宽限：预置契约被打 pending 标记保留（不真删），
	// 提案同样保留，由清扫器最终收口。
	compensated, err := model.NewFunctionContractModel(gameDB).
		FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.NotNil(t, compensated.RemovalPendingAt, "pending 操作补偿应打摘除宽限标记而非真删")
	_, err = model.NewPageProposalModel(gameDB).
		FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	assert.NoError(t, err, "补偿后提案宽限期内保留")
	var operation registry.AgentRegistrationOperationDB
	require.NoError(t, metaDB.Where("operation_id = ?", "pending-recovery").First(&operation).Error)
	assert.Equal(t, "compensated", operation.Status)
}

func openRegistrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&registry.AgentSessionDB{},
		&registry.AgentRegistrationOperationDB{},
		&model.FunctionContract{},
		&model.FunctionContractVersion{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
	))
	return db
}

// Removed 无幸存者：注册面函数消失走摘除宽限——契约行保留并标记
// removal_pending_at（目录即时 0 实例、调用 503），宽限内重注册自动
// 清除 pending；宽限过期由清扫（FinalizeExpiredContractRemovals）真删
// 契约并级联清理 standalone 提案。
func TestUpsertAgentRemovalGraceLifecycle(t *testing.T) {
	db := openRegistrationTestDB(t)
	store := registry.NewStoreWithDB(db)
	contractSvc := contractsvc.NewContractService(db)
	store.SetContractService(contractSvc)
	ctx := context.Background()

	session := &registry.AgentSession{
		AgentID:  "agent-removal",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Hour),
		LastSeen: time.Now(),
		Functions: map[string]registry.FunctionMeta{
			"mail.send": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"message":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"ok":{"type":"boolean"}}}`,
			},
		},
	}
	require.NoError(t, store.UpsertAgent(session))
	_, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:mail.send")
	require.NoError(t, err, "standalone 提案应在首次注册后存在")

	// 同一 agent 重注册为空函数集：函数 Removed 且无幸存者。必须用新
	// 对象——内存 registry 存的是 session 指针，原地清空 Functions 会让
	// previous 快照同步变空、diff 捕捉不到 Removed。
	markEmpty := func() {
		emptySession := *session
		emptySession.Functions = map[string]registry.FunctionMeta{}
		require.NoError(t, store.UpsertAgent(&emptySession))
	}
	markEmpty()

	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "demo-game", "development", "mail.send")
	require.NoError(t, err, "宽限期内契约行保留不真删")
	assert.NotNil(t, contract.RemovalPendingAt, "消失后应标记 removal_pending_at")
	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:mail.send")
	assert.NoError(t, err, "宽限期内 standalone 提案保留")

	// 宽限期内重注册：pending 自动清除。必须传新对象——内存 registry 存
	// 的是 session 指针，原对象回传时 previous 快照与本次注册同源，
	// diff 捕捉不到 Added、清除路径不会触发。
	recovered := *session
	recovered.Functions = map[string]registry.FunctionMeta{
		"mail.send": session.Functions["mail.send"],
	}
	require.NoError(t, store.UpsertAgent(&recovered))
	contract, err = model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.Nil(t, contract.RemovalPendingAt, "宽限内重注册应清除 pending")

	// 再次消失后宽限过期，清扫真删契约并级联清理 standalone 提案，
	// 且留 removed 版本历史。
	markEmpty()
	finalized, err := contractSvc.FinalizeExpiredContractRemovals(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, finalized)

	_, err = model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "demo-game", "development", "mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "宽限过期后契约真删")
	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "真删后 standalone 提案级联清理")

	versions, total, err := model.NewFunctionContractVersionModel(db).ListByFunctionPaged(ctx, "demo-game", "development", "mail.send", 10, 0)
	require.NoError(t, err)
	require.NotZero(t, total, "removed 版本历史应保留")
	assert.Equal(t, "removed", versions[0].ChangeType, "最新一条历史应为 removed（seq DESC 排列）")
}

type transactionalFailingMaterializer struct {
	service *contractsvc.ContractService
}

// contractFailingMaterializer 在契约物化（权威状态）上注入失败：用于验证
// 事务内失败仍回滚整个注册。
type contractFailingMaterializer struct {
	service *contractsvc.ContractService
}

func (m *contractFailingMaterializer) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta spec.FunctionContractInput) error {
	if err := m.service.RebuildContractFromFunctionMeta(ctx, gameID, env, source, meta); err != nil {
		return err
	}
	return errors.New("injected contract failure")
}

func (m *contractFailingMaterializer) RegenerateContractTemplates(ctx context.Context, gameID, env string) error {
	return m.service.RegenerateContractTemplates(ctx, gameID, env)
}

func (m *contractFailingMaterializer) RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (string, error) {
	return m.service.RemoveFunctionContract(ctx, gameID, env, functionID)
}

func (m *contractFailingMaterializer) MarkContractRemovalPending(ctx context.Context, gameID, env, functionID string) error {
	return m.service.MarkContractRemovalPending(ctx, gameID, env, functionID)
}

func (m *contractFailingMaterializer) FinalizeExpiredContractRemovals(ctx context.Context, grace time.Duration) (int, error) {
	return m.service.FinalizeExpiredContractRemovals(ctx, grace)
}

func (m *contractFailingMaterializer) RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	return m.service.RebuildResourceCapability(ctx, gameID, env, resourceKey)
}

func (m *contractFailingMaterializer) RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error {
	return m.service.RebuildProposalsForResource(ctx, gameID, env, resourceKey)
}

func (m *contractFailingMaterializer) RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error {
	return m.service.RebuildProposalForFunction(ctx, gameID, env, functionID)
}

func (m *transactionalFailingMaterializer) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta spec.FunctionContractInput) error {
	return m.service.RebuildContractFromFunctionMeta(ctx, gameID, env, source, meta)
}

func (m *transactionalFailingMaterializer) RegenerateContractTemplates(ctx context.Context, gameID, env string) error {
	return m.service.RegenerateContractTemplates(ctx, gameID, env)
}

func (m *transactionalFailingMaterializer) RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (string, error) {
	return m.service.RemoveFunctionContract(ctx, gameID, env, functionID)
}

func (m *transactionalFailingMaterializer) MarkContractRemovalPending(ctx context.Context, gameID, env, functionID string) error {
	return m.service.MarkContractRemovalPending(ctx, gameID, env, functionID)
}

func (m *transactionalFailingMaterializer) FinalizeExpiredContractRemovals(ctx context.Context, grace time.Duration) (int, error) {
	return m.service.FinalizeExpiredContractRemovals(ctx, grace)
}

func (m *transactionalFailingMaterializer) RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	return m.service.RebuildResourceCapability(ctx, gameID, env, resourceKey)
}

// 提案注入直接失败（不执行真实重建）：移出事务后，靠事务回滚抹掉提案
// 的旧语义不再成立，需模拟「提案确实没写进去」的 DB 故障形态。
func (m *transactionalFailingMaterializer) RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error {
	return errors.New("injected proposal failure")
}

func (m *transactionalFailingMaterializer) RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error {
	return errors.New("injected proposal failure")
}
