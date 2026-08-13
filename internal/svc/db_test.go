package svc

import (
	"os"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDriverAndDSN_Extended(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.Config
		envDriver  string
		envDSN     string
		wantDriver string
		wantDSN    string
	}{
		{
			name:       "postgres from key=value format",
			cfg:        config.Config{Database: config.DatabaseConfig{Driver: "postgres", DataSource: "host=localhost dbname=test"}},
			wantDriver: "postgres",
			wantDSN:    "host=localhost dbname=test",
		},
		{
			name:       "mysql from key=value format",
			cfg:        config.Config{Database: config.DatabaseConfig{Driver: "mysql", DataSource: "root:pass@tcp(localhost:3306)/db"}},
			wantDriver: "mysql",
			wantDSN:    "root:pass@tcp(localhost:3306)/db",
		},
		{
			name:       "sqlserver from key=value format",
			cfg:        config.Config{Database: config.DatabaseConfig{Driver: "sqlserver", DataSource: "server=localhost;database=test"}},
			wantDriver: "sqlserver",
			wantDSN:    "server=localhost;database=test",
		},
		{
			name:       "auto with pgx prefix",
			cfg:        config.Config{Database: config.DatabaseConfig{Driver: "auto", DataSource: "pgx://localhost/db"}},
			wantDriver: "postgres",
			wantDSN:    "pgx://localhost/db",
		},
		{
			name:       "auto with mysql prefix",
			cfg:        config.Config{Database: config.DatabaseConfig{Driver: "auto", DataSource: "mysql://localhost/db"}},
			wantDriver: "mysql",
			wantDSN:    "mysql://localhost/db",
		},
		{
			name:       "auto with sqlserver prefix",
			cfg:        config.Config{Database: config.DatabaseConfig{Driver: "auto", DataSource: "sqlserver://localhost/db"}},
			wantDriver: "sqlserver",
			wantDSN:    "sqlserver://localhost/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envDriver != "" {
				t.Setenv("DB_DRIVER", tt.envDriver)
			}
			if tt.envDSN != "" {
				t.Setenv("DATABASE_URL", tt.envDSN)
			}
			driver, dsn := resolveDriverAndDSN(tt.cfg)
			assert.Equal(t, tt.wantDriver, driver)
			assert.Equal(t, tt.wantDSN, dsn)
		})
	}
}

func TestExtractMySQLDatabaseName_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		dsn    string
		expect string
	}{
		{"standard", "user:pass@tcp(localhost:3306)/mydb", "mydb"},
		{"with params", "user:pass@tcp(localhost:3306)/mydb?charset=utf8", "mydb"},
		{"empty", "", ""},
		{"no db", "user:pass@tcp(localhost:3306)/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMySQLDatabaseName(tt.dsn)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestExtractPostgresDatabaseName_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		dsn    string
		expect string
	}{
		{"standard", "postgres://user:pass@localhost:5432/mydb", "mydb"},
		{"with query", "postgres://user:pass@localhost:5432/mydb?sslmode=disable", "mydb"},
		{"postgresql prefix", "postgresql://user:pass@localhost:5432/mydb", "mydb"},
		{"no port", "postgres://user:pass@localhost/mydb", "mydb"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPostgresDatabaseName(tt.dsn)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestRemoveDBFromPostgresDSN_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		dsn    string
		replDB string
		expect string
	}{
		{"standard", "postgres://user:pass@localhost:5432/mydb", "newdb", "postgres://user:pass@localhost:5432/newdb"},
		{"with query", "postgres://user:pass@localhost:5432/mydb?sslmode=disable", "newdb", "postgres://user:pass@localhost:5432/newdb?sslmode=disable"},
		{"postgresql", "postgresql://user:pass@localhost:5432/mydb", "newdb", "postgresql://user:pass@localhost:5432/newdb"},
		{"no match", "host=localhost", "", "host=localhost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeDBFromPostgresDSN(tt.dsn, tt.replDB)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestExtractSQLServerDatabaseName_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		dsn    string
		expect string
	}{
		{"standard", "sqlserver://user:pass@localhost:1433?database=mydb", "mydb"},
		{"semicolon", "server=localhost;user id=sa;password=pass;database=mydb", "mydb"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSQLServerDatabaseName(tt.dsn)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestFirstNonEmpty_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		vals   []string
		expect string
	}{
		{"first", []string{"a", "b", "c"}, "a"},
		{"second", []string{"", "b", "c"}, "b"},
		{"third", []string{"", "", "c"}, "c"},
		{"all empty", []string{"", "", ""}, ""},
		{"empty input", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := firstNonEmpty(tt.vals...)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestParseTelemetryDuration_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect time.Duration
	}{
		{"10s", "10s", 10 * time.Second},
		{"5m", "5m", 5 * time.Minute},
		{"1h", "1h", 1 * time.Hour},
		{"empty", "", 0},
		{"invalid", "abc", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTelemetryDuration(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestReplaceMySQLDSNDB_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		dsn    string
		db     string
		expect string
	}{
		{"standard", "user:pass@tcp(localhost:3306)/mydb", "newdb", "user:pass@tcp(localhost:3306)/newdb"},
		{"with params", "user:pass@tcp(localhost:3306)/mydb?charset=utf8", "newdb", "user:pass@tcp(localhost:3306)/newdb?charset=utf8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceMySQLDSNDB(tt.dsn, tt.db)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestOpenGorm_SQLite(t *testing.T) {
	db, err := openGorm("sqlite", ":memory:")
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
}

func TestOpenGorm_SQLiteEmptyDSN(t *testing.T) {
	db, err := openGorm("sqlite", "")
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
}

func TestOpenGorm_SQLite3(t *testing.T) {
	db, err := openGorm("sqlite3", ":memory:")
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
}

func TestOpenGorm_UnsupportedDriver(t *testing.T) {
	_, err := openGorm("unsupported", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database driver")
}

func TestOpenDatabase(t *testing.T) {
	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "sqlite",
			DataSource: ":memory:",
		},
	}
	db, err := openDatabase(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
}

func TestResolveDriverAndDSN_EnvOverride(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", ":memory:")

	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "postgres",
			DataSource: "host=localhost dbname=test",
		},
	}

	driver, dsn := resolveDriverAndDSN(cfg)
	assert.Equal(t, "sqlite", driver)
	assert.Equal(t, ":memory:", dsn)
}

func TestOpenReadOnlyGorm(t *testing.T) {
	db, err := openReadOnlyGorm("sqlite", ":memory:")
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
}

func TestOpenReadOnlyGorm_UnsupportedDriver(t *testing.T) {
	_, err := openReadOnlyGorm("unsupported", "")
	assert.Error(t, err)
}

func TestSqliteFilePath_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		dsn    string
		expect string
	}{
		{"empty", "", "data/croupier.db"},
		{"simple", "/tmp/test.db", "/tmp/test.db"},
		{"with prefix", "sqlite:///tmp/test.db", "tmp/test.db"},
		{"with file prefix", "file:/tmp/test.db", "/tmp/test.db"},
		{"with query", "/tmp/test.db?mode=ro", "/tmp/test.db"},
		{"relative", "data/test.db", "data/test.db"},
		{"relative with file", "file:data/test.db", "data/test.db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqliteFilePath(tt.dsn)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestSqliteDSNForGame_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		metaDSN string
		dbName  string
		expect  string
	}{
		{"empty meta", "", "game1", "file:game1?mode=memory&cache=shared"},
		{"memory meta", ":memory:", "game1", "file:game1?mode=memory&cache=shared"},
		{"file path with dir", "/tmp/data/meta.db", "game1", "/tmp/data/game1.db"},
		{"file path no dir", "meta.db", "game1", "game1.db"},
		{"sqlite prefix", "sqlite:///tmp/data/meta.db", "game1", "sqlite:///tmp/data/game1.db"},
		{"file prefix", "file:/tmp/data/meta.db", "game1", "file:/tmp/data/game1.db"},
		{"file prefix no dir", "file:meta.db", "game1", "file:game1.db"},
		{"with query", "/tmp/data/meta.db?vfs=memdb", "game1", "/tmp/data/game1.db?vfs=memdb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqliteDSNForGame(tt.metaDSN, tt.dbName)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestEnsureSQLiteFileExists(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name:    "empty",
			dsn:     "",
			wantErr: false,
		},
		{
			name:    "memory",
			dsn:     ":memory:",
			wantErr: false,
		},
		{
			name:    "nonexistent file",
			dsn:     "/tmp/nonexistent_test_db_12345.db",
			wantErr: true,
		},
		{
			name:    "existing file",
			dsn:     "/tmp/existing_test.db",
			wantErr: false,
		},
	}

	// Create a test file
	testFile := "/tmp/existing_test.db"
	f, err := os.Create(testFile)
	if err == nil {
		f.Close()
		defer os.Remove(testFile)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureSQLiteFileExists(tt.dsn)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnsureSQLiteDirExtended(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{"empty", "", false},
		{"memory", ":memory:", false},
		{"simple path", "/tmp/test.db", false},
		{"with sqlite prefix", "sqlite:///tmp/test.db", false},
		{"with file prefix", "file:/tmp/test.db", false},
		{"with query", "/tmp/test.db?mode=ro", false},
		{"relative path", "data/test.db", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureSQLiteDir(tt.dsn)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQuotePostgresIdentifierExtended(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"simple", "testdb", "testdb"},
		{"with hyphen", "test-db", `"test-db"`},
		{"with space", "test db", `"test db"`},
		{"with dot", "test.db", `"test.db"`},
		{"with quotes", `test"db`, `test"db`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := quotePostgresIdentifier(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestRemoveDBFromMySQLDSNExtended(t *testing.T) {
	tests := []struct {
		name   string
		dsn    string
		expect string
	}{
		{"standard", "user:pass@tcp(localhost:3306)/mydb?charset=utf8", "user:pass@tcp(localhost:3306)/?charset=utf8"},
		{"no params", "user:pass@tcp(localhost:3306)/mydb", "user:pass@tcp(localhost:3306)/mydb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeDBFromMySQLDSN(tt.dsn)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestReplaceDBInSQLServerDSNExtended(t *testing.T) {
	tests := []struct {
		name   string
		dsn    string
		db     string
		expect string
	}{
		{"standard", "server=localhost;database=old", "newdb", "server=localhost;database=newdb"},
		{"with params", "server=localhost;database=old;encrypt=true", "newdb", "server=localhost;database=newdb;encrypt=true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceDBInSQLServerDSN(tt.dsn, tt.db)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestDSNForDatabase(t *testing.T) {
	tests := []struct {
		name     string
		driver   string
		metaDSN  string
		dbName   string
		contains string
	}{
		{"sqlite", "sqlite", "/tmp/meta.db", "game1", "game1"},
		{"sqlite3", "sqlite3", "/tmp/meta.db", "game1", "game1"},
		{"postgres", "postgres", "postgres://localhost/meta", "game1", "game1"},
		{"postgresql", "postgresql", "postgresql://localhost/meta", "game1", "game1"},
		{"mysql", "mysql", "root:pass@tcp(localhost:3306)/meta", "game1", "game1"},
		{"sqlserver", "sqlserver", "server=localhost;database=meta", "game1", "game1"},
		{"unknown defaults to postgres", "unknown", "postgres://localhost/meta", "game1", "game1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := DSNForDatabase(tt.driver, tt.metaDSN, tt.dbName)
			if dsn == "" {
				t.Error("DSNForDatabase() returned empty string")
			}
			if !contains(dsn, tt.contains) {
				t.Errorf("DSNForDatabase() = %q, should contain %q", dsn, tt.contains)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
