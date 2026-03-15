package extensiongorm

import (
	"context"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

type EventRepo struct {
	db *gorm.DB
}

type EventListQuery struct {
	InstallationID uint
	Level          string
	Keyword        string
	Limit          int
	Offset         int
}

func NewEventRepo(db *gorm.DB) *EventRepo {
	return &EventRepo{db: db}
}

func (r *EventRepo) Create(ctx context.Context, item *model.ExtensionEvent) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *EventRepo) ListByInstallationID(ctx context.Context, installationID uint, limit, offset int) ([]model.ExtensionEvent, int64, error) {
	return r.List(ctx, EventListQuery{
		InstallationID: installationID,
		Limit:          limit,
		Offset:         offset,
	})
}

func (r *EventRepo) List(ctx context.Context, q EventListQuery) ([]model.ExtensionEvent, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ExtensionEvent{})
	if q.InstallationID > 0 {
		query = query.Where("installation_id = ?", q.InstallationID)
	}
	if q.Level != "" {
		query = query.Where("level = ?", q.Level)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		query = query.Where("event_type LIKE ? OR message LIKE ? OR created_by LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if q.Limit > 0 {
		query = query.Limit(q.Limit)
	}
	if q.Offset > 0 {
		query = query.Offset(q.Offset)
	}
	var items []model.ExtensionEvent
	if err := query.Order("id desc").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
