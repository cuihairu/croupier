package model

import (
	"context"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// RetentionModel manages cohort persistence.
type RetentionModel struct {
	db *gorm.DB
}

// NewRetentionModel creates a retention model helper.
func NewRetentionModel(db *gorm.DB) *RetentionModel {
	return &RetentionModel{db: db}
}

// UpsertCohort stores cohort metrics.
func (m *RetentionModel) UpsertCohort(ctx context.Context, cohort *RetentionCohort) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(cohort).Error
}

// ListCohorts fetches retention cohorts for filters.
func (m *RetentionModel) ListCohorts(ctx context.Context, gameID, env, cohortName string) ([]RetentionCohort, error) {
	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&RetentionCohort{})
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}
	if cohortName != "" {
		query = query.Where("cohort = ?", cohortName)
	}

	var cohorts []RetentionCohort
	if err := query.Order("window_start DESC").Find(&cohorts).Error; err != nil {
		return nil, err
	}

	return cohorts, nil
}
