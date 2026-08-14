package model

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RegistrationWarningDB represents the database model for registration warnings.
type RegistrationWarningDB struct {
	ID         uint      `gorm:"primaryKey"`
	Key        string    `gorm:"size:64;uniqueIndex;not null"`
	GameID     string    `gorm:"size:64;index;not null"`
	Env        string    `gorm:"size:32;index;not null"`
	AgentID    string    `gorm:"size:64;index;not null"`
	FunctionID string    `gorm:"size:128;index;not null"`
	Version    string    `gorm:"size:32"`
	Code       string    `gorm:"size:64;index;not null"`
	Message    string    `gorm:"type:text;not null"`
	Count      int       `gorm:"default:1"`
	Status     string    `gorm:"size:16;index;default:'pending'"` // pending/read/resolved
	FirstSeen  time.Time `gorm:"not null"`
	LastSeen   time.Time `gorm:"not null"`
	ResolvedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

// TableName specifies the table name for RegistrationWarningDB.
func (RegistrationWarningDB) TableName() string {
	return "registration_warnings"
}

// WarningFilter defines filters for listing warnings.
type WarningFilter struct {
	GameID     string
	Env        string
	AgentID    string
	FunctionID string
	Code       string
	Status     string
	Limit      int
}

// RegistrationWarningModel manages registration warning persistence.
type RegistrationWarningModel struct {
	db *gorm.DB
}

// NewRegistrationWarningModel creates a new RegistrationWarningModel.
func NewRegistrationWarningModel(db *gorm.DB) *RegistrationWarningModel {
	return &RegistrationWarningModel{db: db}
}

// Upsert inserts or updates a registration warning.
func (m *RegistrationWarningModel) Upsert(ctx context.Context, warn *registry.FunctionRegistrationWarning) error {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)

	dbWarn := toDBWarning(warn)
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"count", "last_seen", "version", "agent_id", "function_id"}),
	}).Create(dbWarn).Error
}

// List returns warnings with optional filters.
func (m *RegistrationWarningModel) List(ctx context.Context, filter WarningFilter) ([]*registry.FunctionRegistrationWarning, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)

	var dbWarnings []RegistrationWarningDB
	query := db.Where("deleted_at IS NULL")

	if filter.GameID != "" {
		query = query.Where("game_id = ?", filter.GameID)
	}
	if filter.Env != "" {
		query = query.Where("env = ?", filter.Env)
	}
	if filter.AgentID != "" {
		query = query.Where("agent_id = ?", filter.AgentID)
	}
	if filter.FunctionID != "" {
		query = query.Where("function_id = ?", filter.FunctionID)
	}
	if filter.Code != "" {
		query = query.Where("code = ?", filter.Code)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if err := query.Order("last_seen DESC").Find(&dbWarnings).Error; err != nil {
		return nil, err
	}

	warnings := make([]*registry.FunctionRegistrationWarning, 0, len(dbWarnings))
	for _, dw := range dbWarnings {
		warnings = append(warnings, toDomainWarning(&dw))
	}
	return warnings, nil
}

// UpdateStatus updates the status of a warning.
func (m *RegistrationWarningModel) UpdateStatus(ctx context.Context, key string, status string) error {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)

	updates := map[string]interface{}{"status": status}
	if status == "resolved" {
		now := time.Now()
		updates["resolved_at"] = now
	}

	return db.Model(&RegistrationWarningDB{}).
		Where("key = ?", key).
		Updates(updates).Error
}

// DeleteResolved deletes all resolved warnings.
func (m *RegistrationWarningModel) DeleteResolved(ctx context.Context) (int64, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)

	result := db.Where("status = ?", "resolved").Delete(&RegistrationWarningDB{})
	return result.RowsAffected, result.Error
}

// ClearByAgent clears all warnings for a specific agent.
func (m *RegistrationWarningModel) ClearByAgent(ctx context.Context, agentID string) (int64, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)

	result := db.Where("agent_id = ?", agentID).Delete(&RegistrationWarningDB{})
	return result.RowsAffected, result.Error
}

// CountByStatus returns the count of warnings grouped by status.
func (m *RegistrationWarningModel) CountByStatus(ctx context.Context) (map[string]int64, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)

	var results []struct {
		Status string
		Count  int64
	}

	if err := db.Model(&RegistrationWarningDB{}).
		Select("status, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("status").
		Scan(&results).Error; err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.Status] = r.Count
	}
	return counts, nil
}

// toDBWarning converts a domain warning to a database model.
func toDBWarning(warn *registry.FunctionRegistrationWarning) *RegistrationWarningDB {
	return &RegistrationWarningDB{
		Key:        warn.Key,
		GameID:     warn.GameID,
		Env:        warn.Env,
		AgentID:    warn.AgentID,
		FunctionID: warn.FunctionID,
		Version:    warn.Version,
		Code:       warn.Code,
		Message:    warn.Message,
		Count:      warn.Count,
		Status:     "pending",
		FirstSeen:  warn.FirstSeen,
		LastSeen:   warn.LastSeen,
	}
}

// toDomainWarning converts a database model to a domain warning.
func toDomainWarning(dbWarn *RegistrationWarningDB) *registry.FunctionRegistrationWarning {
	return &registry.FunctionRegistrationWarning{
		Key:        dbWarn.Key,
		GameID:     dbWarn.GameID,
		Env:        dbWarn.Env,
		AgentID:    dbWarn.AgentID,
		FunctionID: dbWarn.FunctionID,
		Version:    dbWarn.Version,
		Code:       dbWarn.Code,
		Message:    dbWarn.Message,
		Count:      dbWarn.Count,
		FirstSeen:  dbWarn.FirstSeen,
		LastSeen:   dbWarn.LastSeen,
	}
}
