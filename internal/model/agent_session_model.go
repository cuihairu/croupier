package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentSessionDB represents the database model for agent sessions.
type AgentSessionDB struct {
	ID        uint           `gorm:"primaryKey"`
	AgentID   string         `gorm:"size:64;uniqueIndex;not null"`
	GameID    string         `gorm:"size:64;index"`
	Env       string         `gorm:"size:32;index"`
	RPCAddr   string         `gorm:"size:255;not null"`
	Version   string         `gorm:"size:32"`
	Region    string         `gorm:"size:64;index"`
	Zone      string         `gorm:"size:64;index"`
	Labels    datatypes.JSON `gorm:"type:json"`
	Providers datatypes.JSON `gorm:"type:json"`
	ExpireAt  time.Time      `gorm:"index;not null"`
	LastSeen  time.Time      `gorm:"index;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName specifies the table name for AgentSessionDB.
func (AgentSessionDB) TableName() string {
	return "agent_sessions"
}

// AgentSessionModel manages agent session persistence.
type AgentSessionModel struct {
	db *gorm.DB
}

// NewAgentSessionModel creates a new AgentSessionModel.
func NewAgentSessionModel(db *gorm.DB) *AgentSessionModel {
	return &AgentSessionModel{db: db}
}

// Upsert inserts or updates an agent session.
func (m *AgentSessionModel) Upsert(ctx context.Context, sess *registry.AgentSession) error {
	dbSess, err := toDBSession(sess)
	if err != nil {
		return err
	}

	return m.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "agent_id"}},
			UpdateAll: true,
		}).
		Create(dbSess).Error
}

// LoadActiveSessions loads all active (non-expired, non-deleted) sessions.
func (m *AgentSessionModel) LoadActiveSessions(ctx context.Context) ([]*registry.AgentSession, error) {
	var dbSessions []AgentSessionDB

	err := m.db.WithContext(ctx).
		Where("expire_at > ?", time.Now()).
		Where("deleted_at IS NULL").
		Find(&dbSessions).Error

	if err != nil {
		return nil, err
	}

	sessions := make([]*registry.AgentSession, 0, len(dbSessions))
	for _, dbSess := range dbSessions {
		sess, err := toDomainSession(&dbSess)
		if err != nil {
			// Skip invalid sessions
			continue
		}
		sessions = append(sessions, sess)
	}

	return sessions, nil
}

// DeleteExpired soft-deletes all expired sessions.
func (m *AgentSessionModel) DeleteExpired(ctx context.Context) (int64, error) {
	result := m.db.WithContext(ctx).
		Where("expire_at <= ?", time.Now()).
		Where("deleted_at IS NULL").
		Delete(&AgentSessionDB{})

	return result.RowsAffected, result.Error
}

// toDomainSession converts a DB session to a registry.AgentSession.
func toDomainSession(dbSess *AgentSessionDB) (*registry.AgentSession, error) {
	sess := &registry.AgentSession{
		AgentID:  dbSess.AgentID,
		GameID:   dbSess.GameID,
		Env:      dbSess.Env,
		RPCAddr:  dbSess.RPCAddr,
		Version:  dbSess.Version,
		Region:   dbSess.Region,
		Zone:     dbSess.Zone,
		ExpireAt: dbSess.ExpireAt,
		LastSeen: dbSess.LastSeen,
	}

	// Parse Labels JSON
	if len(dbSess.Labels) > 0 {
		if err := json.Unmarshal(dbSess.Labels, &sess.Labels); err != nil {
			return nil, err
		}
	}

	// Parse Providers JSON
	if len(dbSess.Providers) > 0 {
		if err := json.Unmarshal(dbSess.Providers, &sess.Providers); err != nil {
			return nil, err
		}
	}

	return sess, nil
}

// toDBSession converts a registry.AgentSession to a DB session.
func toDBSession(sess *registry.AgentSession) (*AgentSessionDB, error) {
	dbSess := &AgentSessionDB{
		AgentID:  sess.AgentID,
		GameID:   sess.GameID,
		Env:      sess.Env,
		RPCAddr:  sess.RPCAddr,
		Version:  sess.Version,
		Region:   sess.Region,
		Zone:     sess.Zone,
		ExpireAt: sess.ExpireAt,
		LastSeen: sess.LastSeen,
	}

	// Marshal Labels to JSON
	if sess.Labels != nil {
		labelsJSON, err := json.Marshal(sess.Labels)
		if err != nil {
			return nil, err
		}
		dbSess.Labels = datatypes.JSON(labelsJSON)
	}

	// Marshal Providers to JSON
	if sess.Providers != nil {
		providersJSON, err := json.Marshal(sess.Providers)
		if err != nil {
			return nil, err
		}
		dbSess.Providers = datatypes.JSON(providersJSON)
	}

	return dbSess, nil
}
