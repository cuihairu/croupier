package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var contractTestDBCounter int

// setupContractTestDB creates an in-memory SQLite database for contract model testing.
func setupContractTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	contractTestDBCounter++
	dbName := fmt.Sprintf("file:memdb%d?mode=memory&cache=shared", contractTestDBCounter)
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(
		&FunctionContract{},
		&ResourceCapability{},
		&CapabilitySemantics{},
		&CapabilitySemanticVersion{},
		&PageSpec{},
		&PublishedPageSpec{},
		&PageVersion{},
		&GameEnvBinding{},
	)
	require.NoError(t, err)
	return db
}

// ===== FunctionContractModel Tests =====

func TestNewFunctionContractModel(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewFunctionContractModel(db)
	assert.NotNil(t, m)
}

func TestFunctionContractModel_UpsertContract_Create(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	contract := &FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "player.ban",
		Version:    "1.0.0",
		Enabled:    true,
		Capability: "action",
		Execution:  "task",
		Risk:       "high",
	}
	err := m.UpsertContract(ctx, contract)
	require.NoError(t, err)
	assert.NotZero(t, contract.ID)
}

func TestFunctionContractModel_UpsertContract_Update(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	contract := &FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "player.ban",
		Version:    "1.0.0",
		Enabled:    true,
	}
	require.NoError(t, m.UpsertContract(ctx, contract))
	origID := contract.ID

	// Upsert again with update
	contract.Version = "2.0.0"
	contract.Enabled = false
	err := m.UpsertContract(ctx, contract)
	require.NoError(t, err)
	assert.Equal(t, origID, contract.ID, "should preserve original ID")
}

func TestFunctionContractModel_FindByScopeAndFunctionID(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	contract := &FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "player.ban",
		Version:    "1.0.0",
	}
	require.NoError(t, m.UpsertContract(ctx, contract))

	found, err := m.FindByScopeAndFunctionID(ctx, "game1", "prod", "player.ban")
	require.NoError(t, err)
	assert.Equal(t, "player.ban", found.FunctionID)

	_, err = m.FindByScopeAndFunctionID(ctx, "game1", "prod", "nonexistent")
	assert.Error(t, err)
}

func TestFunctionContractModel_ListByScope(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	require.NoError(t, m.UpsertContract(ctx, &FunctionContract{
		GameID: "game1", Env: "prod", FunctionID: "player.ban",
	}))
	require.NoError(t, m.UpsertContract(ctx, &FunctionContract{
		GameID: "game1", Env: "prod", FunctionID: "player.kick",
	}))
	require.NoError(t, m.UpsertContract(ctx, &FunctionContract{
		GameID: "game1", Env: "dev", FunctionID: "player.ban",
	}))

	contracts, err := m.ListByScope(ctx, "game1", "prod")
	require.NoError(t, err)
	assert.Len(t, contracts, 2)
}

func TestFunctionContractModel_ListByResourceKey(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	require.NoError(t, m.UpsertContract(ctx, &FunctionContract{
		GameID: "game1", Env: "prod", FunctionID: "player.ban", ResourceKey: "player",
	}))
	require.NoError(t, m.UpsertContract(ctx, &FunctionContract{
		GameID: "game1", Env: "prod", FunctionID: "player.kick", ResourceKey: "player",
	}))

	contracts, err := m.ListByResourceKey(ctx, "game1", "prod", "player")
	require.NoError(t, err)
	assert.Len(t, contracts, 2)
}

func TestFunctionContractModel_DeleteByScopeAndFunctionID(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	require.NoError(t, m.UpsertContract(ctx, &FunctionContract{
		GameID: "game1", Env: "prod", FunctionID: "player.ban",
	}))

	err := m.DeleteByScopeAndFunctionID(ctx, "game1", "prod", "player.ban")
	require.NoError(t, err)

	_, err = m.FindByScopeAndFunctionID(ctx, "game1", "prod", "player.ban")
	assert.Error(t, err)
}

// ===== ResourceCapabilityModel Tests =====

func TestNewResourceCapabilityModel(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewResourceCapabilityModel(db)
	assert.NotNil(t, m)
}

func TestResourceCapabilityModel_UpsertCapability_Create(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewResourceCapabilityModel(db)
	ctx := context.Background()

	cap := &ResourceCapability{
		GameID:      "game1",
		Env:         "prod",
		ResourceKey: "player",
	}
	err := m.UpsertCapability(ctx, cap)
	require.NoError(t, err)
	assert.NotZero(t, cap.ID)
}

func TestResourceCapabilityModel_UpsertCapability_Update(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewResourceCapabilityModel(db)
	ctx := context.Background()

	cap := &ResourceCapability{
		GameID:      "game1",
		Env:         "prod",
		ResourceKey: "player",
		CategoryKey: "user",
	}
	require.NoError(t, m.UpsertCapability(ctx, cap))
	origID := cap.ID

	cap.CategoryKey = "account"
	require.NoError(t, m.UpsertCapability(ctx, cap))
	assert.Equal(t, origID, cap.ID)
}

func TestResourceCapabilityModel_FindByScopeAndResourceKey(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewResourceCapabilityModel(db)
	ctx := context.Background()

	require.NoError(t, m.UpsertCapability(ctx, &ResourceCapability{
		GameID: "game1", Env: "prod", ResourceKey: "player",
	}))

	found, err := m.FindByScopeAndResourceKey(ctx, "game1", "prod", "player")
	require.NoError(t, err)
	assert.Equal(t, "player", found.ResourceKey)

	_, err = m.FindByScopeAndResourceKey(ctx, "game1", "prod", "nonexistent")
	assert.Error(t, err)
}

func TestResourceCapabilityModel_ListByScope(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewResourceCapabilityModel(db)
	ctx := context.Background()

	require.NoError(t, m.UpsertCapability(ctx, &ResourceCapability{
		GameID: "game1", Env: "prod", ResourceKey: "player",
	}))
	require.NoError(t, m.UpsertCapability(ctx, &ResourceCapability{
		GameID: "game1", Env: "prod", ResourceKey: "order",
	}))

	caps, err := m.ListByScope(ctx, "game1", "prod")
	require.NoError(t, err)
	assert.Len(t, caps, 2)
}

// ===== CapabilitySemanticsModel Tests =====

func TestNewCapabilitySemanticsModel(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewCapabilitySemanticsModel(db)
	assert.NotNil(t, m)
}

func TestCapabilitySemanticsModel_UpsertSemantics_Create(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewCapabilitySemanticsModel(db)
	ctx := context.Background()

	sem := &CapabilitySemantics{
		GameID:      "game1",
		Env:         "prod",
		ResourceKey: "player",
	}
	err := m.UpsertSemantics(ctx, sem)
	require.NoError(t, err)
	assert.Equal(t, 1, sem.Version)
}

func TestCapabilitySemanticsModel_UpsertSemantics_Update(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewCapabilitySemanticsModel(db)
	ctx := context.Background()

	sem := &CapabilitySemantics{
		GameID:      "game1",
		Env:         "prod",
		ResourceKey: "player_upd",
	}
	require.NoError(t, m.UpsertSemantics(ctx, sem))
	assert.Equal(t, 1, sem.Version)

	sem.IdentityField = "id"
	require.NoError(t, m.UpsertSemantics(ctx, sem))
	assert.Equal(t, 2, sem.Version)
}

func TestCapabilitySemanticsModel_FindByScopeAndResourceKey(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewCapabilitySemanticsModel(db)
	ctx := context.Background()

	require.NoError(t, m.UpsertSemantics(ctx, &CapabilitySemantics{
		GameID: "game1", Env: "prod", ResourceKey: "player",
	}))

	found, err := m.FindByScopeAndResourceKey(ctx, "game1", "prod", "player")
	require.NoError(t, err)
	assert.Equal(t, "player", found.ResourceKey)

	_, err = m.FindByScopeAndResourceKey(ctx, "game1", "prod", "nonexistent")
	assert.Error(t, err)
}

func TestCapabilitySemanticsModel_ListByScope(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewCapabilitySemanticsModel(db)
	ctx := context.Background()

	require.NoError(t, m.UpsertSemantics(ctx, &CapabilitySemantics{
		GameID: "game1", Env: "prod", ResourceKey: "player_lscope",
	}))
	require.NoError(t, m.UpsertSemantics(ctx, &CapabilitySemantics{
		GameID: "game1", Env: "prod", ResourceKey: "order_lscope",
	}))

	sems, err := m.ListByScope(ctx, "game1", "prod")
	require.NoError(t, err)
	assert.Len(t, sems, 2)
}

func TestCapabilitySemanticsModel_Update(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewCapabilitySemanticsModel(db)
	ctx := context.Background()

	require.NoError(t, m.UpsertSemantics(ctx, &CapabilitySemantics{
		GameID: "game1", Env: "prod", ResourceKey: "player",
	}))

	sem, err := m.FindByScopeAndResourceKey(ctx, "game1", "prod", "player")
	require.NoError(t, err)

	sem.IdentityField = "player_id"
	sem.IdentityFieldType = "string"
	err = m.Update(ctx, sem)
	require.NoError(t, err)

	found, err := m.FindByScopeAndResourceKey(ctx, "game1", "prod", "player")
	require.NoError(t, err)
	assert.Equal(t, "player_id", found.IdentityField)
}

// ===== CapabilitySemanticVersionModel Tests =====

func TestNewCapabilitySemanticVersionModel(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewCapabilitySemanticVersionModel(db)
	assert.NotNil(t, m)
}

func TestCapabilitySemanticVersionModel_CreateVersion(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewCapabilitySemanticVersionModel(db)
	ctx := context.Background()

	// Create semantics first
	semModel := NewCapabilitySemanticsModel(db)
	require.NoError(t, semModel.UpsertSemantics(ctx, &CapabilitySemantics{
		GameID: "game1", Env: "prod", ResourceKey: "player",
	}))
	sem, _ := semModel.FindByScopeAndResourceKey(ctx, "game1", "prod", "player")

	ver := &CapabilitySemanticVersion{
		SemanticsID: sem.ID,
		Version:     1,
		Semantics:   datatypes.JSON(`{"identityField":"player_id"}`),
	}
	err := m.CreateVersion(ctx, ver)
	require.NoError(t, err)
	assert.NotZero(t, ver.ID)
}

func TestCapabilitySemanticVersionModel_ListBySemanticsID(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewCapabilitySemanticVersionModel(db)
	ctx := context.Background()

	// Create semantics
	semModel := NewCapabilitySemanticsModel(db)
	require.NoError(t, semModel.UpsertSemantics(ctx, &CapabilitySemantics{
		GameID: "game1", Env: "prod", ResourceKey: "player_vlist",
	}))
	sem, _ := semModel.FindByScopeAndResourceKey(ctx, "game1", "prod", "player_vlist")

	require.NoError(t, m.CreateVersion(ctx, &CapabilitySemanticVersion{
		SemanticsID: sem.ID, Version: 1, Semantics: datatypes.JSON(`{}`),
	}))
	require.NoError(t, m.CreateVersion(ctx, &CapabilitySemanticVersion{
		SemanticsID: sem.ID, Version: 2, Semantics: datatypes.JSON(`{}`),
	}))

	vers, err := m.ListBySemanticsID(ctx, sem.ID)
	require.NoError(t, err)
	assert.Len(t, vers, 2)
	// Should be ordered by version DESC
	assert.GreaterOrEqual(t, vers[0].Version, vers[1].Version)
}

// ===== PageSpec Tests =====

func TestPageSpecModel_Delete(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewPageSpecModel(db)
	ctx := context.Background()

	ps := &PageSpec{
		GameID:  "game1",
		Env:     "prod",
		PageKey: "resource--player",
		Status:  "active",
	}
	require.NoError(t, m.Upsert(ctx, ps))

	err := m.Delete(ctx, "game1", "prod", "resource--player")
	require.NoError(t, err)

	found, err := m.FindByScopeAndPageKey(ctx, "game1", "prod", "resource--player")
	assert.Error(t, err)
	assert.Nil(t, found)
}

func TestPageSpecModel_ListByScopeAndStatus(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewPageSpecModel(db)
	ctx := context.Background()

	require.NoError(t, m.Upsert(ctx, &PageSpec{
		GameID: "game1", Env: "prod", PageKey: "p1", Status: "active",
	}))
	require.NoError(t, m.Upsert(ctx, &PageSpec{
		GameID: "game1", Env: "prod", PageKey: "p2", Status: "draft",
	}))

	items, err := m.ListByScopeAndStatus(ctx, "game1", "prod", "active")
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "p1", items[0].PageKey)
}

// ===== PublishedPageSpecModel Tests =====

func TestPublishedPageSpecModel_FindByScopePageKeyAndVersion(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewPublishedPageSpecModel(db)
	ctx := context.Background()

	ps := &PublishedPageSpec{
		GameID:  "game1",
		Env:     "prod",
		PageKey: "resource--player",
		Version: 1,
		Active:  true,
	}
	require.NoError(t, m.Create(ctx, ps))

	found, err := m.FindByScopePageKeyAndVersion(ctx, "game1", "prod", "resource--player", 1)
	require.NoError(t, err)
	assert.Equal(t, "resource--player", found.PageKey)

	_, err = m.FindByScopePageKeyAndVersion(ctx, "game1", "prod", "resource--player", 99)
	assert.Error(t, err)
}

func TestPublishedPageSpecModel_ListByScope(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewPublishedPageSpecModel(db)
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, &PublishedPageSpec{
		GameID: "list_scope", Env: "prod", PageKey: "list_p1", Version: 1, Active: true,
	}))
	require.NoError(t, m.Create(ctx, &PublishedPageSpec{
		GameID: "list_scope", Env: "prod", PageKey: "list_p2", Version: 1, Active: false,
	}))

	items, err := m.ListByScope(ctx, "list_scope", "prod")
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestPublishedPageSpecModel_ListLatestActiveByScope(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewPublishedPageSpecModel(db)
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, &PublishedPageSpec{
		GameID: "latest_scope", Env: "prod", PageKey: "latest_p1", Version: 1, Active: true,
	}))
	require.NoError(t, m.Create(ctx, &PublishedPageSpec{
		GameID: "latest_scope", Env: "prod", PageKey: "latest_p1", Version: 2, Active: true,
	}))
	// Create a non-active record by creating it then deactivating
	inactive := &PublishedPageSpec{
		GameID: "latest_scope", Env: "prod", PageKey: "latest_p2", Version: 1, Active: true,
	}
	require.NoError(t, m.Create(ctx, inactive))
	require.NoError(t, m.DeactivatePage(ctx, "latest_scope", "prod", "latest_p2", time.Now()))

	// Returns all active records (latest_p1 v1 and v2 are active, latest_p2 is not)
	items, err := m.ListLatestActiveByScope(ctx, "latest_scope", "prod")
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestPublishedPageSpecModel_DeactivatePage(t *testing.T) {
	db := setupContractTestDB(t)
	m := NewPublishedPageSpecModel(db)
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, &PublishedPageSpec{
		GameID: "deact_scope", Env: "prod", PageKey: "deact_p1", Version: 1, Active: true,
	}))

	err := m.DeactivatePage(ctx, "deact_scope", "prod", "deact_p1", time.Now())
	require.NoError(t, err)

	found, err := m.FindLatestByScopeAndPageKey(ctx, "deact_scope", "prod", "deact_p1")
	assert.Error(t, err)
	assert.Nil(t, found)
}

// ===== GameEnvBinding Tests =====

func TestGameEnvBindingModel_Create(t *testing.T) {
	db := setupContractTestDB(t)
	ctx := context.Background()

	binding := &GameEnvBinding{
		GameID:       "game1",
		Env:          "prod",
		DatabaseName: "game1_prod",
	}
	err := db.WithContext(ctx).Create(binding).Error
	require.NoError(t, err)
	assert.NotZero(t, binding.ID)
}
