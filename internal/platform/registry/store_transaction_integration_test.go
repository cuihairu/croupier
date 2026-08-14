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
	store.SetContractService(&transactionalFailingMaterializer{service: contractsvc.NewContractService(db)})
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
	assert.ErrorContains(t, err, "injected proposal failure")

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
	store.SetContractService(&transactionalFailingMaterializer{service: contractsvc.NewContractService(db)})
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

	_, err = model.NewFunctionContractModel(gameDB).
		FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = model.NewPageProposalModel(gameDB).
		FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var operation registry.AgentRegistrationOperationDB
	require.NoError(t, metaDB.Where("agent_id = ?", "agent-cross-db").First(&operation).Error)
	assert.Equal(t, "compensated", operation.Status)
	store.Mu().RLock()
	assert.Nil(t, store.AgentsUnsafe()["agent-cross-db"])
	store.Mu().RUnlock()

	// Retrying the exact snapshot after the meta store recovers creates one
	// consistent session/projection pair, and a fresh process restores it.
	require.NoError(t, metaDB.AutoMigrate(&registry.AgentSessionDB{}))
	require.NoError(t, store.UpsertAgent(session))
	_, err = model.NewFunctionContractModel(gameDB).
		FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	require.NoError(t, err)
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

	_, err = model.NewFunctionContractModel(gameDB).
		FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = model.NewPageProposalModel(gameDB).
		FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
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
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
	))
	return db
}

type transactionalFailingMaterializer struct {
	service *contractsvc.ContractService
}

func (m *transactionalFailingMaterializer) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta spec.FunctionContractInput) error {
	return m.service.RebuildContractFromFunctionMeta(ctx, gameID, env, source, meta)
}

func (m *transactionalFailingMaterializer) RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (string, error) {
	return m.service.RemoveFunctionContract(ctx, gameID, env, functionID)
}

func (m *transactionalFailingMaterializer) RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	return m.service.RebuildResourceCapability(ctx, gameID, env, resourceKey)
}

func (m *transactionalFailingMaterializer) RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error {
	if err := m.service.RebuildProposalsForResource(ctx, gameID, env, resourceKey); err != nil {
		return err
	}
	return errors.New("injected proposal failure")
}

func (m *transactionalFailingMaterializer) RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error {
	if err := m.service.RebuildProposalForFunction(ctx, gameID, env, functionID); err != nil {
		return err
	}
	return errors.New("injected proposal failure")
}
