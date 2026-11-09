package svc

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/pressly/goose/v3"
)

// adminLoginSecurityMigration 为存量库补 admins 登录安全列（0017）：
// failed_attempts/locked_until（T1 登录失败锁定）与 token_version
// （T2 token 撤销）一次补齐，避免两条迁移改同一张表。新库由 baseline
// AutoMigrate 直接带出；幂等：列已存在则跳过。admins 是 meta 库表，
// fanout 重放到 game 库时表不存在则直接跳过（与 0015 同理）。
func adminLoginSecurityMigration() *goose.Migration {
	return goose.NewGoMigration(17,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			if !migrator.HasTable(&model.Admin{}) {
				return nil
			}
			for _, col := range []string{"FailedAttempts", "LockedUntil", "TokenVersion"} {
				if migrator.HasColumn(&model.Admin{}, col) {
					continue
				}
				if err := migrator.AddColumn(&model.Admin{}, col); err != nil {
					return fmt.Errorf("migrate: 0017 add admins.%s: %w", col, err)
				}
			}
			return nil
		}},
		nil,
	)
}

// adminMfaMigration 为存量库补 admins.otp_enabled 列（0018）；新库由
// baseline AutoMigrate 带出；幂等；game 库无 admins 表直接跳过。
func adminMfaMigration() *goose.Migration {
	return goose.NewGoMigration(18,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			if !migrator.HasTable(&model.Admin{}) {
				return nil
			}
			if migrator.HasColumn(&model.Admin{}, "OTPEnabled") {
				return nil
			}
			if err := migrator.AddColumn(&model.Admin{}, "OTPEnabled"); err != nil {
				return fmt.Errorf("migrate: 0018 add admins.otp_enabled: %w", err)
			}
			return nil
		}},
		nil,
	)
}

// roleAdminSoftDeleteCleanupMigration 为 0026：roles/admins 删除路径改硬删
// （Unscoped）之后，把 0024 同族漏网的存量软删行物理清除。roles.name 与
// admins.username 是物理唯一索引，软删行占着索引位会导致同名重建直接
// duplicate-key 500（T-M9 菜单 E2E 实证：删除角色后重建报 UNIQUE
// constraint failed: roles.name）。纯 DELETE，不动表结构，幂等；缺表/缺列跳过。
// admin_roles 里指向被清行的悬挂关联一并清除（软删时代就存在，借迁移收口）。
func roleAdminSoftDeleteCleanupMigration() *goose.Migration {
	return goose.NewGoMigration(26,
		&goose.GoFunc{RunDB: migrateRoleAdminSoftDelete},
		nil,
	)
}

// migrateRoleAdminSoftDelete 是 0026 的迁移体（抽出便于直测）。
func migrateRoleAdminSoftDelete(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	targets := []struct {
		table string
		model interface{}
	}{
		{"roles", &model.Role{}},
		{"admins", &model.Admin{}},
	}
	for _, target := range targets {
		if !db.Migrator().HasTable(target.model) {
			continue
		}
		if !db.Migrator().HasColumn(target.model, "DeletedAt") {
			continue
		}
		if err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE deleted_at IS NOT NULL", target.table)).Error; err != nil {
			return fmt.Errorf("migrate: 0026 purge soft-deleted rows from %s: %w", target.table, err)
		}
	}
	if db.Migrator().HasTable(&model.AdminRole{}) {
		if err := db.Exec(`DELETE FROM admin_roles WHERE role_id NOT IN (SELECT id FROM roles)`).Error; err != nil {
			return fmt.Errorf("migrate: 0026 purge dangling admin_roles: %w", err)
		}
	}
	return nil
}

// adminOtpRecoveryCodesMigration 为 0032：建 admin_otp_recovery_codes 表。
//
// 新建表，无存量数据约束；幂等（表已存在则跳过）。与 0028/0030 同模式：不整
// 模型 AutoMigrate——存量库上会与既有约束名漂移 panic（0023 教训）。
func adminOtpRecoveryCodesMigration() *goose.Migration {
	return goose.NewGoMigration(32,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			if migrator.HasTable(&model.AdminOTPRecoveryCode{}) {
				return nil
			}
			if err := migrator.CreateTable(&model.AdminOTPRecoveryCode{}); err != nil {
				return fmt.Errorf("migrate: 0032 create admin_otp_recovery_codes: %w", err)
			}
			return nil
		}},
		nil,
	)
}

// adminPasswordPolicyMigration 为存量库补 admins 密码策略列（0033）：
// must_change_password（登录后必须修改密码）+ password_expires_at（密码
// 有效期截止，NULL=长期有效）。新库由 baseline AutoMigrate 带出；逐列
// HasColumn+AddColumn 幂等（0017/0018 加列同模式）；game 库无 admins 表
// 直接跳过。
func adminPasswordPolicyMigration() *goose.Migration {
	return goose.NewGoMigration(33,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			if !migrator.HasTable(&model.Admin{}) {
				return nil
			}
			for _, col := range []string{"MustChangePassword", "PasswordExpiresAt"} {
				if migrator.HasColumn(&model.Admin{}, col) {
					continue
				}
				if err := migrator.AddColumn(&model.Admin{}, col); err != nil {
					return fmt.Errorf("migrate: 0033 add admins.%s: %w", col, err)
				}
			}
			return nil
		}},
		nil,
	)
}

// emailVerificationMigration 为 0037：建 email_verifications 表（注册邮箱
// 验证令牌，#51c 第二批）+ admins 逐列补 email_verified。新表无存量数据
// （HasTable 后 CreateTable，0032/0035/0036 同模式）；改既有表走
// HasColumn+AddColumn 逐列补齐（0033 密码策略列同模式；不整模型
// AutoMigrate——存量约束名漂移 panic，0023 教训）。
func emailVerificationMigration() *goose.Migration {
	return goose.NewGoMigration(37,
		&goose.GoFunc{RunDB: migrateEmailVerification},
		nil,
	)
}

// migrateEmailVerification 是 0037 的迁移体（抽出便于直测）。
func migrateEmailVerification(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&model.EmailVerification{}) {
		if err := migrator.CreateTable(&model.EmailVerification{}); err != nil {
			return fmt.Errorf("migrate: 0037 create email_verifications: %w", err)
		}
	}
	if !migrator.HasTable(&model.Admin{}) {
		return nil // game 库无 admins 表：缺表跳过，fanout 不报错
	}
	if migrator.HasColumn(&model.Admin{}, "EmailVerified") {
		return nil
	}
	if err := migrator.AddColumn(&model.Admin{}, "EmailVerified"); err != nil {
		return fmt.Errorf("migrate: 0037 add admins.email_verified: %w", err)
	}
	return nil
}
