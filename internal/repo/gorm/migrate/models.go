//go:build legacy_repo
// +build legacy_repo

package migrate

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// MigrationRecord tracks all database migrations
type MigrationRecord struct {
	gorm.Model
	Name        string     `gorm:"primaryKey;size:128;not null"` // Migration name
	Version     string     `gorm:"size:32;not null"`             // Semantic version
	Description string     `gorm:"type:text"`                    // Migration description
	SQL         string     `gorm:"type:text"`                    // Migration SQL
	Checksum    string     `gorm:"size:64;not null"`             // MD5 checksum of migration
	ExecutedAt  *time.Time // When migration was executed
	ExecutedBy  string     `gorm:"size:64"`                 // Who executed it (system, admin)
	Status      string     `gorm:"size:16;default:pending"` // pending, success, failed
}

// TableName returns the table name for MigrationRecord
func (MigrationRecord) TableName() string {
	return "migration_records"
}

// MigrationStep represents a single migration step
type MigrationStep struct {
	Number      int    `json:"number"`      // Step number in order
	Name        string `json:"name"`        // Step name
	Description string `json:"description"` // Step description
	SQL         string `json:"sql"`         // SQL to execute
	RollbackSQL string `json:"rollbackSql"` // SQL to rollback
	DryRun      bool   `json:"dryRun"`      // Whether this is a dry run
}

// MigrationDirection represents migration direction
type MigrationDirection string

const (
	DirectionUp   MigrationDirection = "up"
	DirectionDown MigrationDirection = "down"
)

// MigrationStatus represents migration status
type MigrationStatus string

const (
	StatusPending MigrationStatus = "pending"
	StatusSuccess MigrationStatus = "success"
	StatusFailed  MigrationStatus = "failed"
)

// MigrationResult represents the result of a migration execution
type MigrationResult struct {
	MigrationName string             `json:"migrationName"`
	Direction     MigrationDirection `json:"direction"`
	Status        MigrationStatus    `json:"status"`
	Error         string             `json:"error,omitempty"`
	Duration      string             `json:"duration"`
	SQL           string             `json:"sql"`
	DryRun        bool               `json:"dryRun"`
}

// MigrationFunc represents a migration function
type MigrationFunc func(db *gorm.DB) error

// Migration interface
type Migration interface {
	Name() string
	Version() string
	Description() string
	Up(db *gorm.DB) error
	Down(db *gorm.DB) error
}

// BaseMigration provides common migration functionality
type BaseMigration struct {
	name        string
	version     string
	description string
	upSQL       string
	downSQL     string
}

// NewBaseMigration creates a new base migration
func NewBaseMigration(name, version, description, upSQL, downSQL string) *BaseMigration {
	return &BaseMigration{
		name:        name,
		version:     version,
		description: description,
		upSQL:       upSQL,
		downSQL:     downSQL,
	}
}

// Name implements Migration interface
func (m *BaseMigration) Name() string {
	return m.name
}

// Version implements Migration interface
func (m *BaseMigration) Version() string {
	return m.version
}

// Description implements Migration interface
func (m *BaseMigration) Description() string {
	return m.description
}

// Up implements Migration interface
func (m *BaseMigration) Up(db *gorm.DB) error {
	if m.upSQL == "" {
		return nil
	}
	return db.Exec(m.upSQL).Error
}

// Down implements Migration interface
func (m *BaseMigration) Down(db *gorm.DB) error {
	if m.downSQL == "" {
		return nil
	}
	return db.Exec(m.downSQL).Error
}
