package svc

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cuihairu/croupier/internal/config"
	gsqlite "github.com/glebarez/sqlite"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	gsqlserver "gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openDatabase(cfg config.Config) (*gorm.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	dsn := strings.TrimSpace(cfg.Database.DataSource)

	// Allow env to override config for dev/CI.
	// See docs: DB_DRIVER, DATABASE_URL.
	if envDriver := strings.TrimSpace(os.Getenv("DB_DRIVER")); envDriver != "" {
		driver = strings.ToLower(envDriver)
	}
	if envDSN := strings.TrimSpace(os.Getenv("DATABASE_URL")); envDSN != "" {
		dsn = envDSN
	}

	if driver == "" {
		driver = "auto"
	}
	if driver == "auto" {
		switch {
		case strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "pgx://"):
			driver = "postgres"
		case strings.HasPrefix(dsn, "mysql://"):
			driver = "mysql"
		case strings.HasPrefix(dsn, "sqlserver://"):
			driver = "sqlserver"
		default:
			driver = "sqlite"
		}
	}

	switch driver {
	case "sqlite", "sqlite3":
		if dsn == "" {
			dsn = "data/croupier.db"
		}
		if err := ensureSQLiteDir(dsn); err != nil {
			return nil, err
		}
		return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	case "postgres", "postgresql", "pg":
		if dsn == "" {
			return nil, fmt.Errorf("postgres DSN is required")
		}
		return gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	case "mysql":
		if dsn == "" {
			return nil, fmt.Errorf("mysql DSN is required")
		}
		// 尝试连接，如果数据库不存在则创建
		db, err := gorm.Open(gmysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			// 检查是否是数据库不存在的错误
			if strings.Contains(err.Error(), "Unknown database") {
				// 从 DSN 中提取数据库名称
				dbName := extractMySQLDatabaseName(dsn)
				if dbName == "" {
					return nil, fmt.Errorf("failed to extract database name from DSN: %w", err)
				}
				// 连接到 MySQL 服务器（不指定数据库）
				dsnWithoutDB := removeDBFromMySQLDSN(dsn)
				if dsnWithoutDB == "" {
					return nil, fmt.Errorf("failed to remove database name from DSN: %w", err)
				}
				// 创建数据库
				if err := createMySQLDatabase(dsnWithoutDB, dbName); err != nil {
					return nil, fmt.Errorf("failed to create database %s: %w", dbName, err)
				}
				// 重新连接
				db, err = gorm.Open(gmysql.Open(dsn), &gorm.Config{})
				if err != nil {
					return nil, fmt.Errorf("failed to connect after creating database: %w", err)
				}
			} else {
				return nil, err
			}
		}
		return db, nil
	case "sqlserver":
		if dsn == "" {
			return nil, fmt.Errorf("sqlserver DSN is required")
		}
		return gorm.Open(gsqlserver.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

func ensureSQLiteDir(dsn string) error {
	if dsn == "" || dsn == ":memory:" {
		return nil
	}

	// Common SQLite DSNs:
	// - data/croupier.db
	// - file:data/croupier.db?cache=shared
	// - sqlite:///abs/path/to.db
	// - :memory:
	if strings.HasPrefix(dsn, "sqlite:///") {
		dsn = strings.TrimPrefix(dsn, "sqlite:///")
	}
	if strings.HasPrefix(dsn, "file:") {
		dsn = strings.TrimPrefix(dsn, "file:")
	}
	if idx := strings.IndexByte(dsn, '?'); idx >= 0 {
		dsn = dsn[:idx]
	}
	if dsn == "" || dsn == ":memory:" {
		return nil
	}

	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// extractMySQLDatabaseName 从 MySQL DSN 中提取数据库名称
// 例如: "root:root@tcp(localhost:3306)/croupier?..." -> "croupier"
func extractMySQLDatabaseName(dsn string) string {
	// DSN 格式: user:password@tcp(host:port)/dbname?params
	re := regexp.MustCompile(`/([^/?]+)`)
	matches := re.FindStringSubmatch(dsn)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// removeDBFromMySQLDSN 从 MySQL DSN 中移除数据库名称
// 例如: "root:root@tcp(localhost:3306)/croupier?..." -> "root:root@tcp(localhost:3306)/?..."
func removeDBFromMySQLDSN(dsn string) string {
	// 替换 /dbname/ 为 /
	re := regexp.MustCompile(`/[^/?]+(\?)`)
	return re.ReplaceAllString(dsn, "/$1")
}

// createMySQLDatabase 创建 MySQL 数据库
func createMySQLDatabase(dsn, dbName string) error {
	// 使用标准 database/sql 包创建数据库
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	// 创建数据库（如果不存在）
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName))
	return err
}
