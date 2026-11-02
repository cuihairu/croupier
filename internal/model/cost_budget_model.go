package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// CostBudget 是 OPS-007 月度预算配置（每 game/env 一条）。
type CostBudget struct {
	ID         uint   `gorm:"primaryKey"`
	GameID     string `gorm:"size:64;uniqueIndex:idx_cost_budget_scope;not null"`
	Env        string `gorm:"size:64;uniqueIndex:idx_cost_budget_scope;not null"`
	MonthlyUSD float64
	// NotifyChannelsJSON 通知渠道（站内/herald 等），JSON 数组。
	NotifyChannelsJSON string `gorm:"type:text"`
	UpdatedBy          string `gorm:"size:64"`
	UpdatedAt          time.Time
}

func (CostBudget) TableName() string { return "cost_budgets" }

type CostBudgetModel struct{ db *gorm.DB }

func NewCostBudgetModel(db *gorm.DB) *CostBudgetModel { return &CostBudgetModel{db: db} }

func (m *CostBudgetModel) Upsert(ctx context.Context, b *CostBudget) error {
	if b.GameID == "" || b.Env == "" {
		return errors.New("gameId and env required")
	}
	return m.db.WithContext(ctx).Where("game_id=? AND env=?", b.GameID, b.Env).
		Assign(map[string]any{"monthly_usd": b.MonthlyUSD, "notify_channels_json": b.NotifyChannelsJSON, "updated_by": b.UpdatedBy, "updated_at": time.Now().UTC()}).
		FirstOrCreate(b).Error
}

func (m *CostBudgetModel) Get(ctx context.Context, gameID, env string) (*CostBudget, error) {
	var b CostBudget
	err := m.db.WithContext(ctx).Where("game_id=? AND env=?", gameID, env).Take(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}
