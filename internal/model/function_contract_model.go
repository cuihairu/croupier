package model

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// FunctionContractModel wraps data access for function contracts.
type FunctionContractModel struct {
	db *gorm.DB
}

// NewFunctionContractModel creates the helper.
func NewFunctionContractModel(db *gorm.DB) *FunctionContractModel {
	return &FunctionContractModel{db: db}
}

// UpsertContract creates or updates a function contract.
func (m *FunctionContractModel) UpsertContract(ctx context.Context, contract *FunctionContract) error {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var existing FunctionContract
	err := db.Where("game_id = ? AND env = ? AND function_id = ?",
		contract.GameID, contract.Env, contract.FunctionID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(contract).Error
	}
	if err != nil {
		return err
	}
	contract.ID = existing.ID
	contract.CreatedAt = existing.CreatedAt
	return db.Save(contract).Error
}

// FindByScopeAndFunctionID fetches a contract by scope and function ID.
func (m *FunctionContractModel) FindByScopeAndFunctionID(ctx context.Context, gameID, env, functionID string) (*FunctionContract, error) {
	var contract FunctionContract
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
		First(&contract).Error; err != nil {
		return nil, err
	}
	return &contract, nil
}

// ListByScope lists all contracts in a scope.
func (m *FunctionContractModel) ListByScope(ctx context.Context, gameID, env string) ([]*FunctionContract, error) {
	var contracts []*FunctionContract
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("function_id").
		Find(&contracts).Error; err != nil {
		return nil, err
	}
	return contracts, nil
}

// ListByResourceKey lists contracts for a resource.
func (m *FunctionContractModel) ListByResourceKey(ctx context.Context, gameID, env, resourceKey string) ([]*FunctionContract, error) {
	var contracts []*FunctionContract
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, resourceKey).
		Order("function_id").
		Find(&contracts).Error; err != nil {
		return nil, err
	}
	return contracts, nil
}

// DeleteByScopeAndFunctionID removes a contract.
func (m *FunctionContractModel) DeleteByScopeAndFunctionID(ctx context.Context, gameID, env, functionID string) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
		Delete(&FunctionContract{}).Error
}

// ResourceCapabilityModel wraps data access for resource capabilities.
type ResourceCapabilityModel struct {
	db *gorm.DB
}

// NewResourceCapabilityModel creates the helper.
func NewResourceCapabilityModel(db *gorm.DB) *ResourceCapabilityModel {
	return &ResourceCapabilityModel{db: db}
}

// UpsertCapability creates or updates a resource capability.
func (m *ResourceCapabilityModel) UpsertCapability(ctx context.Context, cap *ResourceCapability) error {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var existing ResourceCapability
	err := db.Where("game_id = ? AND env = ? AND resource_key = ?",
		cap.GameID, cap.Env, cap.ResourceKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(cap).Error
	}
	if err != nil {
		return err
	}
	cap.ID = existing.ID
	cap.CreatedAt = existing.CreatedAt
	return db.Save(cap).Error
}

// FindByScopeAndResourceKey fetches by scope and resource key.
func (m *ResourceCapabilityModel) FindByScopeAndResourceKey(ctx context.Context, gameID, env, resourceKey string) (*ResourceCapability, error) {
	var cap ResourceCapability
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, resourceKey).
		First(&cap).Error; err != nil {
		return nil, err
	}
	return &cap, nil
}

// ListByScope lists all resource capabilities in a scope.
func (m *ResourceCapabilityModel) ListByScope(ctx context.Context, gameID, env string) ([]*ResourceCapability, error) {
	var caps []*ResourceCapability
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("resource_key").
		Find(&caps).Error; err != nil {
		return nil, err
	}
	return caps, nil
}

// CapabilitySemanticsModel wraps data access for capability semantics.
type CapabilitySemanticsModel struct {
	db *gorm.DB
}

// NewCapabilitySemanticsModel creates the helper.
func NewCapabilitySemanticsModel(db *gorm.DB) *CapabilitySemanticsModel {
	return &CapabilitySemanticsModel{db: db}
}

// UpsertSemantics creates or updates capability semantics.
func (m *CapabilitySemanticsModel) UpsertSemantics(ctx context.Context, sem *CapabilitySemantics) error {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var existing CapabilitySemantics
	err := db.Where("game_id = ? AND env = ? AND resource_key = ?",
		sem.GameID, sem.Env, sem.ResourceKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		sem.Version = 1
		return db.Create(sem).Error
	}
	if err != nil {
		return err
	}
	sem.ID = existing.ID
	sem.CreatedAt = existing.CreatedAt
	sem.Version = existing.Version + 1
	return db.Save(sem).Error
}

// FindByScopeAndResourceKey fetches by scope and resource key.
func (m *CapabilitySemanticsModel) FindByScopeAndResourceKey(ctx context.Context, gameID, env, resourceKey string) (*CapabilitySemantics, error) {
	var sem CapabilitySemantics
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, resourceKey).
		First(&sem).Error; err != nil {
		return nil, err
	}
	return &sem, nil
}

// ListByScope lists all capability semantics in a scope.
func (m *CapabilitySemanticsModel) ListByScope(ctx context.Context, gameID, env string) ([]*CapabilitySemantics, error) {
	var sems []*CapabilitySemantics
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("resource_key").
		Find(&sems).Error; err != nil {
		return nil, err
	}
	return sems, nil
}

// Update updates a capability semantics record.
func (m *CapabilitySemanticsModel) Update(ctx context.Context, sem *CapabilitySemantics) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Save(sem).Error
}

// CapabilitySemanticVersionModel wraps data access for semantic versions.
type CapabilitySemanticVersionModel struct {
	db *gorm.DB
}

// NewCapabilitySemanticVersionModel creates the helper.
func NewCapabilitySemanticVersionModel(db *gorm.DB) *CapabilitySemanticVersionModel {
	return &CapabilitySemanticVersionModel{db: db}
}

// CreateVersion creates a new semantic version record.
func (m *CapabilitySemanticVersionModel) CreateVersion(ctx context.Context, ver *CapabilitySemanticVersion) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(ver).Error
}

// ListBySemanticsID lists versions for a semantics.
func (m *CapabilitySemanticVersionModel) ListBySemanticsID(ctx context.Context, semanticsID uint) ([]*CapabilitySemanticVersion, error) {
	var vers []*CapabilitySemanticVersion
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("semantics_id = ?", semanticsID).
		Order("version DESC").
		Find(&vers).Error; err != nil {
		return nil, err
	}
	return vers, nil
}
