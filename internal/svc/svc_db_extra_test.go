package svc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenGorm_RequiresDSNForServerDrivers(t *testing.T) {
	for _, driver := range []string{"postgres", "postgresql", "pg", "mysql", "sqlserver"} {
		t.Run(driver, func(t *testing.T) {
			_, err := openGorm(driver, "")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "DSN is required")
		})
	}
}

func TestOpenGorm_SQLiteCreatesFile(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "nested", "croupier.db")
	db, err := openGorm("sqlite", dsn)
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	_, err = os.Stat(dsn)
	assert.NoError(t, err)
}

func TestOpenGorm_PostgresInvalidDSN(t *testing.T) {
	// A syntactically invalid DSN fails at open time without any network I/O.
	_, err := openGorm("postgres", "not a valid postgres dsn")
	assert.Error(t, err)
}

func TestOpenGorm_MySQLInvalidDSN(t *testing.T) {
	_, err := openGorm("mysql", "invalid mysql dsn without dsn syntax")
	assert.Error(t, err)
}

func TestOpenGorm_SQLServerInvalidDSN(t *testing.T) {
	_, err := openGorm("sqlserver", "not a valid sqlserver dsn")
	assert.Error(t, err)
}

func TestOpenReadOnlyGorm_DriverMatrix(t *testing.T) {
	t.Run("missing sqlite file", func(t *testing.T) {
		_, err := openReadOnlyGorm("sqlite", filepath.Join(t.TempDir(), "missing.db"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sqlite database does not exist")
	})

	t.Run("existing sqlite file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "present.db")
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		db, err := openReadOnlyGorm("sqlite", path)
		if db != nil {
			sqlDB, _ := db.DB()
			defer sqlDB.Close()
		}
		assert.NoError(t, err)
	})

	for _, driver := range []string{"postgres", "mysql", "sqlserver"} {
		t.Run(driver+" requires dsn", func(t *testing.T) {
			_, err := openReadOnlyGorm(driver, "")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "DSN is required")
		})
	}

	t.Run("postgres invalid dsn", func(t *testing.T) {
		_, err := openReadOnlyGorm("postgres", "not a valid postgres dsn")
		assert.Error(t, err)
	})
}

func TestEnsureSQLiteDir_RejectsFileParent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o644))

	err := ensureSQLiteDir(filepath.Join(filePath, "sub", "db.sqlite"))
	assert.Error(t, err)
}

func TestCreateDatabaseHelpers_UnreachableServers(t *testing.T) {
	// Closed loopback ports fail fast with connection refused; no external
	// network or server process is required.
	t.Run("mysql", func(t *testing.T) {
		err := createMySQLDatabase("root:pass@tcp(127.0.0.1:1)/?charset=utf8mb4", "newdb")
		assert.Error(t, err)
	})

	t.Run("postgres", func(t *testing.T) {
		err := createPostgresDatabase("host=127.0.0.1 port=1 user=postgres password=x sslmode=disable", "newdb")
		assert.Error(t, err)
	})

	t.Run("sqlserver", func(t *testing.T) {
		err := createSQLServerDatabase("sqlserver://sa:pass@127.0.0.1:1?database=master", "newdb")
		assert.Error(t, err)
	})
}

func TestEnsureGameDatabase_SQLiteAndUnsupported(t *testing.T) {
	baseDir := t.TempDir()

	gameDSN, err := EnsureGameDatabase("sqlite", filepath.Join(baseDir, "meta.db"), "game_demo_prod")
	require.NoError(t, err)
	assert.Contains(t, gameDSN, "game_demo_prod.db")

	gameDSN, err = EnsureGameDatabase("sqlite3", "", "game_x")
	require.NoError(t, err)
	assert.NotEmpty(t, gameDSN)

	_, err = EnsureGameDatabase("unknown-driver", "whatever", "game_x")
	assert.Error(t, err)
}

func TestEnsureGameDatabase_ServerDriversFailFast(t *testing.T) {
	t.Run("postgres invalid dsn", func(t *testing.T) {
		_, err := EnsureGameDatabase("postgres", "not a valid postgres dsn", "game_x")
		assert.Error(t, err)
	})

	t.Run("mysql invalid dsn", func(t *testing.T) {
		_, err := EnsureGameDatabase("mysql", "invalid mysql dsn", "game_x")
		assert.Error(t, err)
	})

	t.Run("sqlserver invalid dsn", func(t *testing.T) {
		_, err := EnsureGameDatabase("sqlserver", "not valid either", "game_x")
		assert.Error(t, err)
	})
}
