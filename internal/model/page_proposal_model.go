package model

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// PageProposalModel wraps data access for page proposals.
type PageProposalModel struct {
	db *gorm.DB
}

// NewPageProposalModel creates the helper.
func NewPageProposalModel(db *gorm.DB) *PageProposalModel {
	return &PageProposalModel{db: db}
}

// UpsertProposal creates or updates a page proposal.
func (m *PageProposalModel) UpsertProposal(ctx context.Context, proposal *PageProposal) error {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var existing PageProposal
	err := db.Where("game_id = ? AND env = ? AND proposal_key = ?",
		proposal.GameID, proposal.Env, proposal.ProposalKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(proposal).Error
	}
	if err != nil {
		return err
	}
	proposal.ID = existing.ID
	proposal.CreatedAt = existing.CreatedAt
	return db.Save(proposal).Error
}

// FindByScopeAndKey fetches a proposal by scope and key.
func (m *PageProposalModel) FindByScopeAndKey(ctx context.Context, gameID, env, proposalKey string) (*PageProposal, error) {
	var proposal PageProposal
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND proposal_key = ?", gameID, env, proposalKey).
		First(&proposal).Error; err != nil {
		return nil, err
	}
	return &proposal, nil
}

// FindByScopeAndPageKey fetches the latest proposal that generated a page key.
func (m *PageProposalModel) FindByScopeAndPageKey(ctx context.Context, gameID, env, pageKey string) (*PageProposal, error) {
	var proposal PageProposal
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND page_key = ?", gameID, env, pageKey).
		Order("updated_at DESC, id DESC").
		First(&proposal).Error; err != nil {
		return nil, err
	}
	return &proposal, nil
}

// ListByScope lists all proposals in a scope.
func (m *PageProposalModel) ListByScope(ctx context.Context, gameID, env string) ([]*PageProposal, error) {
	var proposals []*PageProposal
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("proposal_key").
		Find(&proposals).Error; err != nil {
		return nil, err
	}
	return proposals, nil
}

// ListByStatus lists proposals by status.
func (m *PageProposalModel) ListByStatus(ctx context.Context, gameID, env, status string) ([]*PageProposal, error) {
	var proposals []*PageProposal
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND status = ?", gameID, env, status).
		Order("proposal_key").
		Find(&proposals).Error; err != nil {
		return nil, err
	}
	return proposals, nil
}

// ListByScopeAndResourceKey lists proposals for a resource in a scope.
func (m *PageProposalModel) ListByScopeAndResourceKey(ctx context.Context, gameID, env, resourceKey string) ([]*PageProposal, error) {
	var proposals []*PageProposal
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, resourceKey).
		Order("proposal_key").
		Find(&proposals).Error; err != nil {
		return nil, err
	}
	return proposals, nil
}

// ListByScopeStatusAndResourceKey lists proposals by status and resource in a scope.
func (m *PageProposalModel) ListByScopeStatusAndResourceKey(ctx context.Context, gameID, env, status, resourceKey string) ([]*PageProposal, error) {
	var proposals []*PageProposal
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND status = ? AND resource_key = ?", gameID, env, status, resourceKey).
		Order("proposal_key").
		Find(&proposals).Error; err != nil {
		return nil, err
	}
	return proposals, nil
}

// DeleteByScopeAndKey removes a proposal.
func (m *PageProposalModel) DeleteByScopeAndKey(ctx context.Context, gameID, env, proposalKey string) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND proposal_key = ?", gameID, env, proposalKey).
		Delete(&PageProposal{}).Error
}

// PageProposalVersionModel wraps data access for proposal versions.
type PageProposalVersionModel struct {
	db *gorm.DB
}

// NewPageProposalVersionModel creates the helper.
func NewPageProposalVersionModel(db *gorm.DB) *PageProposalVersionModel {
	return &PageProposalVersionModel{db: db}
}

// CreateVersion creates a new proposal version record.
func (m *PageProposalVersionModel) CreateVersion(ctx context.Context, ver *PageProposalVersion) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(ver).Error
}

// ListByProposalID lists versions for a proposal.
func (m *PageProposalVersionModel) ListByProposalID(ctx context.Context, proposalID uint) ([]*PageProposalVersion, error) {
	var vers []*PageProposalVersion
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("proposal_id = ?", proposalID).
		Order("version DESC").
		Find(&vers).Error; err != nil {
		return nil, err
	}
	return vers, nil
}

// FindByProposalIDAndVersion returns one proposal snapshot.
func (m *PageProposalVersionModel) FindByProposalIDAndVersion(ctx context.Context, proposalID uint, version int) (*PageProposalVersion, error) {
	var ver PageProposalVersion
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("proposal_id = ? AND version = ?", proposalID, version).
		First(&ver).Error; err != nil {
		return nil, err
	}
	return &ver, nil
}

// LatestByProposalID returns the latest proposal snapshot.
func (m *PageProposalVersionModel) LatestByProposalID(ctx context.Context, proposalID uint) (*PageProposalVersion, error) {
	var ver PageProposalVersion
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("proposal_id = ?", proposalID).
		Order("version DESC").
		First(&ver).Error; err != nil {
		return nil, err
	}
	return &ver, nil
}

// GetNextVersion returns the next monotonically increasing proposal version.
func (m *PageProposalVersionModel) GetNextVersion(ctx context.Context, proposalID uint) (int, error) {
	var maxVersion int
	err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Model(&PageProposalVersion{}).
		Where("proposal_id = ?", proposalID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error
	return maxVersion + 1, err
}
