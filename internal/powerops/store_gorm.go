package powerops

import (
	"errors"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// GormStore is the SQL RequestStore (power_requests + power_request_targets,
// migration 0064). It takes a plain *gorm.DB — game-scope routing (dbctx) is
// resolved by the caller, mirroring the batchop store pattern.
type GormStore struct {
	db *gorm.DB
}

func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

func (s *GormStore) Create(r *model.PowerRequest) error {
	return s.db.Create(r).Error
}

func (s *GormStore) Update(r *model.PowerRequest) error {
	return s.db.Save(r).Error
}

func (s *GormStore) Get(requestID string) (*model.PowerRequest, error) {
	var row model.PowerRequest
	err := s.db.Where("request_id = ?", requestID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("powerops: get request: %w", err)
	}
	return &row, nil
}

func (s *GormStore) List(limit int) ([]*model.PowerRequest, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var rows []*model.PowerRequest
	err := s.db.Order("id DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("powerops: list requests: %w", err)
	}
	return rows, nil
}

// ReplaceTargets rewrites the per-machine rows transactionally (delete +
// insert keeps the row set equal to the in-memory slice, so partial verify
// refreshes never leave stale rows).
func (s *GormStore) ReplaceTargets(requestID string, targets []*model.PowerRequestTarget) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("request_id = ?", requestID).Delete(&model.PowerRequestTarget{}).Error; err != nil {
			return fmt.Errorf("powerops: clear targets: %w", err)
		}
		for _, t := range targets {
			if err := tx.Create(t).Error; err != nil {
				return fmt.Errorf("powerops: persist target %s: %w", t.NodeID, err)
			}
		}
		return nil
	})
}

func (s *GormStore) Targets(requestID string) ([]*model.PowerRequestTarget, error) {
	var rows []*model.PowerRequestTarget
	err := s.db.Where("request_id = ?", requestID).Order("id ASC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("powerops: list targets: %w", err)
	}
	return rows, nil
}
