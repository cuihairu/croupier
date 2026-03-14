package model

import (
	"gorm.io/gorm"
)

// MigrateAgentSessions runs auto migration for agent sessions table.
func MigrateAgentSessions(db *gorm.DB) error {
	return db.AutoMigrate(&AgentSessionDB{})
}
