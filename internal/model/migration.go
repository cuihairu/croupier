package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// AutoMigrate runs gorm auto migration for all server-owned tables.
func AutoMigrate(db *gorm.DB) error {
	if err := renameLegacyTables(db); err != nil {
		return err
	}

	// For postgres, auto-migration can get stuck on legacy unique constraint names.
	// Attempt a few self-healing iterations before giving up.
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres" {
		var lastErr error
		for range 5 {
			if err := autoMigrateModels(db); err == nil {
				if err := migrateFunctionOpenAPIColumns(db); err != nil {
					return err
				}
				return nil
			} else if tryFixPostgresMissingConstraint(db, err) {
				lastErr = err
				continue
			} else {
				return err
			}
		}
		if lastErr != nil {
			return lastErr
		}
		if err := autoMigrateModels(db); err != nil {
			return err
		}
		return migrateFunctionOpenAPIColumns(db)
	}

	if err := autoMigrateModels(db); err != nil {
		return err
	}
	return migrateFunctionOpenAPIColumns(db)
}

func autoMigrateModels(db *gorm.DB) error {
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
		&AgentSessionDB{},
		&TermDictionary{},
		&WorkspaceConfig{},
	)
}

var postgresMissingConstraintRe = regexp.MustCompile(`constraint "([^"]+)" of relation "([^"]+)" does not exist`)

func tryFixPostgresMissingConstraint(db *gorm.DB, err error) bool {
	if db == nil || err == nil {
		return false
	}
	var unwrapped error = err
	for unwrapped != nil {
		msg := unwrapped.Error()
		matches := postgresMissingConstraintRe.FindStringSubmatch(msg)
		if len(matches) == 3 {
			constraintName := matches[1]
			tableName := matches[2]
			if strings.HasPrefix(constraintName, "uni_") && tableName != "" {
				// Be conservative: only touch legacy "uni_*" names.
				// Some databases may have a postgres-default unique constraint name like:
				//   <table>_<column>_key
				// while GORM expects:
				//   uni_<table>_<column>
				// In that case, rename the existing constraint so the subsequent drop succeeds.
				if tryRenamePostgresDefaultUniqueConstraint(db, tableName, constraintName) {
					return true
				}

				// Otherwise, ensure a missing legacy object doesn't hard-fail the migration.
				// Drop constraint first (if exists), then drop an identically-named index (if exists).
				_ = db.Exec(
					fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s",
						quoteIdent(tableName),
						quoteIdent(constraintName),
					),
				).Error
				_ = db.Exec(
					fmt.Sprintf("DROP INDEX IF EXISTS %s",
						quoteIdent(constraintName),
					),
				).Error
				return true
			}
			return false
		}
		unwrapped = errors.Unwrap(unwrapped)
	}
	return false
}

func tryRenamePostgresDefaultUniqueConstraint(db *gorm.DB, tableName, expectedConstraintName string) bool {
	if db == nil || tableName == "" || expectedConstraintName == "" {
		return false
	}
	prefix := "uni_" + tableName + "_"
	if !strings.HasPrefix(expectedConstraintName, prefix) {
		return false
	}
	columnName := strings.TrimPrefix(expectedConstraintName, prefix)
	if columnName == "" {
		return false
	}
	legacyName := tableName + "_" + columnName + "_key"

	sqlDB, ok := postgresSQLDB(db)
	if !ok {
		return false
	}

	if postgresHasConstraint(sqlDB, tableName, legacyName) && !postgresHasConstraint(sqlDB, tableName, expectedConstraintName) {
		if err := postgresExec(
			sqlDB,
			fmt.Sprintf("ALTER TABLE %s RENAME CONSTRAINT %s TO %s",
				quoteIdent(tableName),
				quoteIdent(legacyName),
				quoteIdent(expectedConstraintName),
			),
		); err == nil {
			return true
		}
	}
	return false
}

func postgresSQLDB(db *gorm.DB) (*sql.DB, bool) {
	if db == nil {
		return nil, false
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB == nil {
		return nil, false
	}
	return sqlDB, true
}

func postgresHasConstraint(sqlDB *sql.DB, tableName, constraintName string) bool {
	if sqlDB == nil || tableName == "" || constraintName == "" {
		return false
	}
	var exists bool
	if err := sqlDB.QueryRowContext(
		context.Background(),
		"select exists(select 1 from pg_constraint where conrelid = $1::regclass and conname = $2)",
		tableName,
		constraintName,
	).Scan(&exists); err != nil {
		return false
	}
	return exists
}

func postgresExec(sqlDB *sql.DB, query string) error {
	if sqlDB == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	_, err := sqlDB.ExecContext(context.Background(), query)
	return err
}

func quoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
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

// migrateFunctionOpenAPIColumns backfills OpenAPI columns from older migration variants.
func migrateFunctionOpenAPIColumns(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&Function{}) {
		return nil
	}

	hasSpecFormat := db.Migrator().HasColumn(&Function{}, "spec_format")
	hasOpenAPISpec := db.Migrator().HasColumn(&Function{}, "open_api_spec")
	hasLegacyOperation := db.Migrator().HasColumn(&Function{}, "openapi_operation")

	if hasOpenAPISpec && hasLegacyOperation {
		if err := db.Exec(`
			UPDATE functions
			SET open_api_spec = COALESCE(open_api_spec, openapi_operation)
			WHERE openapi_operation IS NOT NULL
		`).Error; err != nil {
			return fmt.Errorf("backfill open_api_spec from openapi_operation: %w", err)
		}
	}

	if hasSpecFormat {
		if hasOpenAPISpec {
			if err := db.Exec(`
				UPDATE functions
				SET spec_format = 'openapi3.0.3'
				WHERE (spec_format IS NULL OR spec_format = '')
				  AND open_api_spec IS NOT NULL
			`).Error; err != nil {
				return fmt.Errorf("set spec_format for openapi rows: %w", err)
			}
		}
		if err := db.Exec(`
			UPDATE functions
			SET spec_format = 'legacy'
			WHERE spec_format IS NULL OR spec_format = ''
		`).Error; err != nil {
			return fmt.Errorf("set legacy spec_format default: %w", err)
		}
	}

	return nil
}
