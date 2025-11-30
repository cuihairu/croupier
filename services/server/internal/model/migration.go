package model

import (
	"fmt"

	"gorm.io/gorm"
)

// AutoMigrate runs gorm auto migration for all server-owned tables.
func AutoMigrate(db *gorm.DB) error {
	if err := renameLegacyTables(db); err != nil {
		return err
	}

	return db.AutoMigrate(
		&Admin{},
		&Role{},
		&Permission{},
		&AdminRole{},
		&RolePermission{},
		&AdminGameScope{},
		&AdminGameEnvScope{},
		&Entity{},
		&Game{},
		&Player{},
		&ProfilePermission{},
		&ProfileGame{},
		&Function{},
		&FunctionDescriptor{},
		&Descriptor{},
		&FunctionInstance{},
		&FunctionPermission{},
		&PendingFunction{},
		&Alert{},
		&AlertSilence{},
		&BehaviorEvent{},
		&FeatureAdoption{},
		&PaymentTransaction{},
		&ProductTrend{},
		&RetentionCohort{},
		&Backup{},
		&FAQ{},
		&FAQCategory{},
		&Feedback{},
		&SupportTicket{},
		&SupportComment{},
		&SupportFAQ{},
		&SupportFeedback{},
		&Ticket{},
		&TicketComment{},
		&RateLimit{},
		&Node{},
		&NodeCommand{},
		&Message{},
		&Certificate{},
		&CertificateAlert{},
		&ConfigVersion{},
	)
}

func renameLegacyTables(db *gorm.DB) error {
	type rename struct {
		oldName string
		newName string
	}

	migrator := db.Migrator()
	for _, entry := range []rename{
		{oldName: "admin_records", newName: "admins"},
		{oldName: "role_records", newName: "roles"},
		{oldName: "permission_records", newName: "permissions"},
		{oldName: "admin_role_records", newName: "admin_roles"},
		{oldName: "role_perm_records", newName: "role_permissions"},
	} {
		if entry.oldName == entry.newName {
			continue
		}
		if migrator.HasTable(entry.oldName) && !migrator.HasTable(entry.newName) {
			if err := migrator.RenameTable(entry.oldName, entry.newName); err != nil {
				return fmt.Errorf("rename table %s -> %s: %w", entry.oldName, entry.newName, err)
			}
		}
	}
	return nil
}
