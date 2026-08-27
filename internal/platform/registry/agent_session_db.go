package registry

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentSessionDB represents the database model for agent sessions.
// RPCAddr is retained as a legacy compatibility column until a schema migration removes it.
type AgentSessionDB struct {
	ID        uint      `gorm:"primaryKey"`
	AgentID   string    `gorm:"size:64;uniqueIndex;not null"`
	GameID    string    `gorm:"size:64;index"`
	Env       string    `gorm:"size:32;index"`
	Version   string    `gorm:"size:32"`
	Region    string    `gorm:"size:64;index"`
	Zone      string    `gorm:"size:64;index"`
	Labels    string    `gorm:"type:text"`
	Functions string    `gorm:"type:text"`
	Providers string    `gorm:"type:text"`
	ExpireAt  time.Time `gorm:"index;not null"`
	LastSeen  time.Time `gorm:"index;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// AgentRegistrationOperationDB is the durable recovery record for a
// registration that updates the meta session database and a per-game
// projection database. A pending row means the session write did not commit;
// startup recovery restores the recorded previous projection before accepting
// another registration for the same agent.
type AgentRegistrationOperationDB struct {
	ID              uint      `gorm:"primaryKey"`
	OperationID     string    `gorm:"size:64;uniqueIndex;not null"`
	AgentID         string    `gorm:"size:64;index;not null"`
	GameID          string    `gorm:"size:64;index;not null"`
	Env             string    `gorm:"size:32;index;not null"`
	PreviousSession string    `gorm:"type:text"`
	TargetSession   string    `gorm:"type:text;not null"`
	Status          string    `gorm:"size:32;index;not null"`
	LastError       string    `gorm:"type:text"`
	CreatedAt       time.Time `gorm:"index"`
	UpdatedAt       time.Time
}

// TableName specifies the durable registration recovery table name.
func (AgentRegistrationOperationDB) TableName() string {
	return "agent_registration_operations"
}

// TableName specifies the table name for AgentSessionDB.
func (AgentSessionDB) TableName() string {
	return "agent_sessions"
}

// AgentSessionModel manages agent session persistence.
// This type implements the AgentSessionLoader interface.
type AgentSessionModel struct {
	db *gorm.DB
}

// NewAgentSessionModel creates a new AgentSessionModel.
func NewAgentSessionModel(db *gorm.DB) *AgentSessionModel {
	return &AgentSessionModel{db: db}
}

// MigrateAgentSessions runs auto migration for agent sessions table.
func MigrateAgentSessions(db *gorm.DB) error {
	if err := db.AutoMigrate(&AgentSessionDB{}, &AgentRegistrationOperationDB{}); err != nil {
		return err
	}
	// Drop the legacy rpc_addr column. The agent's reachable address now comes
	// from the live TCP session RemoteAddr, not a self-published string.
	if db.Migrator().HasColumn(&AgentSessionDB{}, "rpc_addr") {
		_ = db.Migrator().DropColumn(&AgentSessionDB{}, "rpc_addr")
	}
	return nil
}

// Upsert inserts or updates an agent session.
func (m *AgentSessionModel) Upsert(ctx context.Context, sess *AgentSession) error {
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
func (m *AgentSessionModel) LoadActiveSessions(ctx context.Context) ([]*AgentSession, error) {
	var dbSessions []AgentSessionDB

	err := m.db.WithContext(ctx).
		Where("expire_at > ?", time.Now()).
		Where("deleted_at IS NULL").
		Find(&dbSessions).Error

	if err != nil {
		return nil, err
	}

	sessions := make([]*AgentSession, 0, len(dbSessions))
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
// RPCAddr is restored only as a compatibility mirror for older API surfaces.
func toDomainSession(dbSess *AgentSessionDB) (*AgentSession, error) {
	sess := &AgentSession{
		AgentID:   dbSess.AgentID,
		GameID:    dbSess.GameID,
		Env:       dbSess.Env,
		Version:   dbSess.Version,
		Region:    dbSess.Region,
		Zone:      dbSess.Zone,
		ExpireAt:  dbSess.ExpireAt,
		LastSeen:  dbSess.LastSeen,
		Labels:    map[string]string{},
		Functions: map[string]FunctionMeta{},
	}

	// Parse Labels JSON
	if len(dbSess.Labels) > 0 {
		if err := json.Unmarshal([]byte(dbSess.Labels), &sess.Labels); err != nil {
			return nil, err
		}
	}

	// Parse Functions JSON
	if len(dbSess.Functions) > 0 {
		if err := json.Unmarshal([]byte(dbSess.Functions), &sess.Functions); err != nil {
			return nil, err
		}
	}

	// Parse Providers JSON
	if len(dbSess.Providers) > 0 {
		if err := json.Unmarshal([]byte(dbSess.Providers), &sess.Providers); err != nil {
			return nil, err
		}
	}

	return sess, nil
}

// toDBSession converts a registry.AgentSession to a DB session.
// RPCAddr continues to dual-write into the legacy column for compatibility.
func toDBSession(sess *AgentSession) (*AgentSessionDB, error) {
	dbSess := &AgentSessionDB{
		AgentID:  sess.AgentID,
		GameID:   sess.GameID,
		Env:      sess.Env,
		Version:  sess.Version,
		Region:   sess.Region,
		Zone:     sess.Zone,
		ExpireAt: sess.ExpireAt,
		LastSeen: sess.LastSeen,
	}

	// Marshal Labels/Functions/Providers to JSON. Nil maps/slices marshal to
	// `null` via json.Marshal, but gorm's postgres driver upsert path
	// mishandles string([]byte("null")) — postgres rejects with
	// "invalid input syntax for type json" even though the same value passes
	// a direct psql INSERT. Use explicit empty JSON instead: {} for maps
	// (nil Labels/Functions), [] for slices (nil Providers).
	if sess.Labels != nil {
		labelsJSON, err := json.Marshal(sess.Labels)
		if err != nil {
			return nil, err
		}
		dbSess.Labels = string(labelsJSON)
	} else {
		dbSess.Labels = string([]byte("{}"))
	}

	if sess.Functions != nil {
		functionsJSON, err := json.Marshal(sess.Functions)
		if err != nil {
			return nil, err
		}
		dbSess.Functions = string(functionsJSON)
	} else {
		dbSess.Functions = string([]byte("{}"))
	}

	if sess.Providers != nil {
		providersJSON, err := json.Marshal(sess.Providers)
		if err != nil {
			return nil, err
		}
		dbSess.Providers = string(providersJSON)
	} else {
		dbSess.Providers = string([]byte("[]"))
	}

	return dbSess, nil
}
