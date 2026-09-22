package service

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 摘除宽限清扫的服务层行为：宽限未到期不删、到期真删；同资源仍有存活
// 契约时资源能力与资源页提案保留（与即时移除路径同净效果）。
func TestContractService_FinalizeExpiredRemovals(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	build := func(id, resource, operation, capability, outputSchema string) {
		require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
			ID:           id,
			Version:      "1.0.0",
			Enabled:      true,
			InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"}}}`,
			OutputSchema: outputSchema,
			Resource:     resource,
			Operation:    operation,
			Capability:   capability,
			Execution:    "sync",
		}))
	}
	// 幸存函数是 collection_query（资源页提案的存在前提），被摘函数是 action。
	build("player.ban", "player", "ban", "action", `{"type":"object","properties":{"success":{"type":"boolean"}}}`)
	build("player.list", "player", "list", "collection_query", `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}},"total":{"type":"integer"}}}`)
	require.NoError(t, service.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, service.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))

	contractModel := model.NewFunctionContractModel(db)
	require.NoError(t, service.MarkContractRemovalPending(ctx, "demo-game", "development", "player.ban"))

	// 宽限未到期：不进候选、不删行。
	finalized, err := service.FinalizeExpiredContractRemovals(ctx, 10*time.Minute)
	require.NoError(t, err)
	assert.Zero(t, finalized)
	_, err = contractModel.FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.ban")
	require.NoError(t, err, "宽限未到期契约行保留")

	// 宽限过期：真删目标契约；同资源仍有存活契约（player.list），
	// 资源能力与资源页提案必须保留。
	finalized, err = service.FinalizeExpiredContractRemovals(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, finalized)

	_, err = contractModel.FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.ban")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "宽限过期后契约真删")
	_, err = contractModel.FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.list")
	require.NoError(t, err, "同资源存活契约不受波及")
	_, err = model.NewResourceCapabilityModel(db).FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	assert.NoError(t, err, "资源仍有存活契约时能力聚合保留")
	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	assert.NoError(t, err, "资源仍有存活契约时资源页提案保留")
}

// 资源最后一个契约被清扫：资源能力、语义与资源页提案一并摘除。
func TestContractService_FinalizeExpiredRemovals_LastContractRemovesResourceState(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID:          "player.ban",
		Version:     "1.0.0",
		Enabled:     true,
		InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"}}}`,
		Resource:    "player",
		Operation:   "ban",
		Capability:  "action",
		Execution:   "sync",
	}))
	require.NoError(t, service.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, service.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))
	require.NoError(t, service.MarkContractRemovalPending(ctx, "demo-game", "development", "player.ban"))

	finalized, err := service.FinalizeExpiredContractRemovals(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, finalized)

	_, err = model.NewResourceCapabilityModel(db).FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "资源无存活契约时能力聚合摘除")
	_, err = model.NewCapabilitySemanticsModel(db).FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "资源无存活契约时语义摘除")
	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "资源无存活契约时资源页提案摘除")
}

// 对不存在的契约打标：静默返回（语义等同已删），不创建行。
func TestContractService_MarkRemovalPending_MissingContractIsSilent(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	require.NoError(t, service.MarkContractRemovalPending(ctx, "demo-game", "development", "ghost"))
	_, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "demo-game", "development", "ghost")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "打标不得创建契约行")
}
