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
	driver, dsn := resolveDriverAndDSN(cfg)
	return openGorm(driver, dsn)
}

// resolveDriverAndDSN resolves the effective driver and DSN from config and
// environment overrides (DB_DRIVER, DATABASE_URL).
func resolveDriverAndDSN(cfg config.Config) (string, string) {
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
	return driver, dsn
}

// openGorm opens a *gorm.DB for the given driver and DSN, auto-creating the
// physical database when it does not yet exist (for non-sqlite drivers).
func openGorm(driver, dsn string) (*gorm.DB, error) {

	switch driver {
	case "sqlite", "sqlite3":
		if dsn == "" {
			dsn = "data/croupier.db"
		}
		if err := ensureSQLiteDir(dsn); err != nil {
			return nil, err
		}
		// SQLite 会自动创建数据库文件，无需额外处理
		return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	case "postgres", "postgresql", "pg":
		if dsn == "" {
			return nil, fmt.Errorf("postgres DSN is required")
		}
		// 尝试连接，如果数据库不存在则创建
		db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			// 检查是否是数据库不存在的错误
			if strings.Contains(err.Error(), "database") && strings.Contains(err.Error(), "does not exist") {
				// 从 DSN 中提取数据库名称
				dbName := extractPostgresDatabaseName(dsn)
				if dbName == "" {
					return nil, fmt.Errorf("failed to extract database name from DSN: %w", err)
				}
				// 连接到默认的 postgres 数据库
				dsnWithoutDB := removeDBFromPostgresDSN(dsn, "postgres")
				if dsnWithoutDB == "" {
					return nil, fmt.Errorf("failed to remove database name from DSN: %w", err)
				}
				// 创建数据库
				if err := createPostgresDatabase(dsnWithoutDB, dbName); err != nil {
					return nil, fmt.Errorf("failed to create database %s: %w", dbName, err)
				}
				// 重新连接
				db, err = gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
				if err != nil {
					return nil, fmt.Errorf("failed to connect after creating database: %w", err)
				}
			} else {
				return nil, err
			}
		}
		return db, nil
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
		// 尝试连接，如果数据库不存在则创建
		db, err := gorm.Open(gsqlserver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			// 检查是否是数据库不存在的错误
			if strings.Contains(err.Error(), "Could not locate database") || strings.Contains(err.Error(), "invalid database") {
				// 从 DSN 中提取数据库名称
				dbName := extractSQLServerDatabaseName(dsn)
				if dbName == "" {
					return nil, fmt.Errorf("failed to extract database name from DSN: %w", err)
				}
				// 连接到 master 数据库
				dsnWithoutDB := replaceDBInSQLServerDSN(dsn, "master")
				if dsnWithoutDB == "" {
					return nil, fmt.Errorf("failed to replace database name in DSN: %w", err)
				}
				// 创建数据库
				if err := createSQLServerDatabase(dsnWithoutDB, dbName); err != nil {
					return nil, fmt.Errorf("failed to create database %s: %w", dbName, err)
				}
				// 重新连接
				db, err = gorm.Open(gsqlserver.Open(dsn), &gorm.Config{})
				if err != nil {
					return nil, fmt.Errorf("failed to connect after creating database: %w", err)
				}
			} else {
				return nil, err
			}
		}
		return db, nil
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

// extractPostgresDatabaseName 从 PostgreSQL DSN 中提取数据库名称
// 例如: "host=localhost port=5432 user=postgres password=postgres dbname=croupier..." -> "croupier"
// 例如: "postgres://user:pass@localhost:5432/croupier?..." -> "croupier"
func extractPostgresDatabaseName(dsn string) string {
	// 尝试从 URL 格式中提取
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		// URL 格式: postgres://user:pass@host:port/dbname?params
		re := regexp.MustCompile(`//[^/]+/([^/?]+)`)
		matches := re.FindStringSubmatch(dsn)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	// 尝试从 key=value 格式中提取
	re := regexp.MustCompile(`dbname=([^&\s]+)`)
	matches := re.FindStringSubmatch(dsn)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// removeDBFromPostgresDSN 从 PostgreSQL DSN 中移除数据库名称，替换为指定的数据库
func removeDBFromPostgresDSN(dsn, replacementDB string) string {
	// 处理 URL 格式
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		// 替换数据库名
		re := regexp.MustCompile(`^(postgres(?:ql)?://[^/]+/)[^/?]+`)
		result := re.ReplaceAllString(dsn, "$1"+replacementDB)
		// 确保查询参数保留
		if idx := strings.Index(dsn, "?"); idx > 0 && !strings.Contains(result, "?") {
			result += dsn[idx:]
		}
		return result
	}
	// 处理 key=value 格式
	re := regexp.MustCompile(`dbname=[^&\s]+`)
	result := re.ReplaceAllString(dsn, "dbname="+replacementDB)
	return result
}

// createPostgresDatabase 创建 PostgreSQL 数据库
func createPostgresDatabase(dsn, dbName string) error {
	// 使用 pq 驱动（需要导入）
	// 这里我们使用标准 database/sql 包
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	// 创建数据库（如果不存在），禁用连接池以避免空闲连接问题
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s WITH ENCODING 'UTF8'", quotePostgresIdentifier(dbName)))
	return err
}

// quotePostgresIdentifier 引用 PostgreSQL 标识符（处理特殊字符）
func quotePostgresIdentifier(ident string) string {
	// PostgreSQL 使用双引号引用标识符
	if strings.Contains(ident, "-") || strings.Contains(ident, " ") || strings.Contains(ident, ".") {
		return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
	}
	return ident
}

// extractSQLServerDatabaseName 从 SQL Server DSN 中提取数据库名称
// 例如: "sqlserver://user:pass@localhost?database=croupier" -> "croupier"
func extractSQLServerDatabaseName(dsn string) string {
	// 尝试从 URL 格式中提取
	if strings.HasPrefix(dsn, "sqlserver://") {
		// URL 格式: sqlserver://user:pass@host?database=dbname
		re := regexp.MustCompile(`database=([^&\s]+)`)
		matches := re.FindStringSubmatch(dsn)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	// 尝试_odbc 格式
	re := regexp.MustCompile(`database=([^;{\s]+)`)
	matches := re.FindStringSubmatch(dsn)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// replaceDBInSQLServerDSN 替换 SQL Server DSN 中的数据库名称
func replaceDBInSQLServerDSN(dsn, replacementDB string) string {
	re := regexp.MustCompile(`database=[^&\s;]+`)
	return re.ReplaceAllString(dsn, "database="+replacementDB)
}

// createSQLServerDatabase 创建 SQL Server 数据库
func createSQLServerDatabase(dsn, dbName string) error {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// 创建数据库（如果不存在）
	_, err = db.Exec(fmt.Sprintf("IF NOT EXISTS (SELECT name FROM sys.databases WHERE name='%s') CREATE DATABASE [%s]", dbName, dbName))
	return err
}

// ============================================================================
// Per-game database helpers (used by internal/db/router)
// ============================================================================

// OpenGormForRouter is the DatabaseOpener callback for the router. It opens a
// *gorm.DB for the given driver and DSN without auto-creating the database
// (the router calls EnsureGameDatabase first).
func OpenGormForRouter(driver, dsn string) (*gorm.DB, error) {
	return openGorm(driver, dsn)
}

// DSNForDatabase returns a DSN that points at the given physical database
// name, derived from the meta DSN. For SQLite the database name is used as a
// sibling file (e.g. "data/croupier.db" + "game_demo_prod" →
// "data/game_demo_prod.db").
func DSNForDatabase(driver, metaDSN, dbName string) string {
	switch driver {
	case "sqlite", "sqlite3":
		return sqliteDSNForGame(metaDSN, dbName)
	case "postgres", "postgresql", "pg":
		return removeDBFromPostgresDSN(metaDSN, dbName)
	case "mysql":
		return replaceMySQLDSNDB(metaDSN, dbName)
	case "sqlserver":
		return replaceDBInSQLServerDSN(metaDSN, dbName)
	default:
		return removeDBFromPostgresDSN(metaDSN, dbName)
	}
}

// EnsureGameDatabase creates the physical database named dbName on the server
// reachable from metaDSN, then returns the DSN pointing at it. For SQLite
// this is a no-op (the file is created on open). For server-based drivers it
// connects to an admin database and runs CREATE DATABASE IF NOT EXISTS.
func EnsureGameDatabase(driver, metaDSN, dbName string) (string, error) {
	gameDSN := DSNForDatabase(driver, metaDSN, dbName)
	switch driver {
	case "sqlite", "sqlite3":
		if err := ensureSQLiteDir(gameDSN); err != nil {
			return "", err
		}
		return gameDSN, nil
	case "postgres", "postgresql", "pg":
		// Try connecting first; only create if missing.
		if _, err := gorm.Open(gpostgres.Open(gameDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}); err != nil {
			if strings.Contains(err.Error(), "database") && strings.Contains(err.Error(), "does not exist") {
				adminDSN := removeDBFromPostgresDSN(metaDSN, "postgres")
				if err := createPostgresDatabase(adminDSN, dbName); err != nil {
					return "", fmt.Errorf("create postgres database %s: %w", dbName, err)
				}
			} else {
				return "", err
			}
		}
		return gameDSN, nil
	case "mysql":
		if _, err := gorm.Open(gmysql.Open(gameDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}); err != nil {
			if strings.Contains(err.Error(), "Unknown database") {
				adminDSN := removeDBFromMySQLDSN(metaDSN)
				if err := createMySQLDatabase(adminDSN, dbName); err != nil {
					return "", fmt.Errorf("create mysql database %s: %w", dbName, err)
				}
			} else {
				return "", err
			}
		}
		return gameDSN, nil
	case "sqlserver":
		if _, err := gorm.Open(gsqlserver.Open(gameDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}); err != nil {
			if strings.Contains(err.Error(), "Could not locate database") || strings.Contains(err.Error(), "invalid database") {
				adminDSN := replaceDBInSQLServerDSN(metaDSN, "master")
				if err := createSQLServerDatabase(adminDSN, dbName); err != nil {
					return "", fmt.Errorf("create sqlserver database %s: %w", dbName, err)
				}
			} else {
				return "", err
			}
		}
		return gameDSN, nil
	default:
		return gameDSN, fmt.Errorf("unsupported driver %q", driver)
	}
}

// sqliteDSNForGame replaces the file name in a SQLite meta DSN with a game DB
// file name, preserving the directory and query parameters.
//
//	e.g. "data/croupier.db" + "game_demo_prod" → "data/game_demo_prod.db"
//	     "file:data/croupier.db?cache=shared" + "game_demo_prod" → "file:data/game_demo_prod.db?cache=shared"
func sqliteDSNForGame(metaDSN, dbName string) string {
	if metaDSN == "" || metaDSN == ":memory:" {
		// In-memory meta: fall back to an in-memory game DB (tests only).
		return "file:" + dbName + "?mode=memory&cache=shared"
	}

	raw := metaDSN
	prefix := ""
	query := ""

	if strings.HasPrefix(raw, "sqlite:///") {
		prefix = "sqlite:///"
		raw = strings.TrimPrefix(raw, prefix)
	} else if strings.HasPrefix(raw, "file:") {
		prefix = "file:"
		raw = strings.TrimPrefix(raw, prefix)
	}
	if idx := strings.IndexByte(raw, '?'); idx >= 0 {
		query = raw[idx:]
		raw = raw[:idx]
	}

	dir := filepath.Dir(raw)
	if dir == "." || dir == "" {
		return prefix + dbName + ".db" + query
	}
	return prefix + filepath.Join(dir, dbName+".db") + query
}

// replaceMySQLDSNDB swaps the database name in a MySQL DSN.
//
//	e.g. "user:pass@tcp(host:3306)/croupier_meta?param=1" + "game_demo_prod"
//	     → "user:pass@tcp(host:3306)/game_demo_prod?param=1"
func replaceMySQLDSNDB(dsn, dbName string) string {
	// Strip any "mysql://" scheme prefix for uniform handling.
	scheme := ""
	if strings.HasPrefix(dsn, "mysql://") {
		scheme = "mysql://"
		dsn = strings.TrimPrefix(dsn, scheme)
	}
	re := regexp.MustCompile(`/([^/?]*)(\?)`)
	if loc := re.FindStringSubmatchIndex(dsn); loc != nil {
		return dsn[:loc[2]+1] + dbName + dsn[loc[3]:]
	}
	// No query string: the path segment is everything after the last '/'.
	if idx := strings.LastIndexByte(dsn, '/'); idx >= 0 {
		return scheme + dsn[:idx+1] + dbName
	}
	return scheme + dsn + "/" + dbName
}
