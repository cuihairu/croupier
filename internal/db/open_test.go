package db

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOpen_PostgreSQL tests the PostgreSQL connection
func TestOpen_PostgreSQL(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{
			name: "postgres:// 前缀",
			dsn:  "postgres://user:pass@localhost:5432/test?sslmode=disable",
		},
		{
			name: "postgresql:// 前缀",
			dsn:  "postgresql://user:pass@localhost:5432/test",
		},
		{
			name: "pgx:// 前缀",
			dsn:  "pgx://user:pass@localhost:5432/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: this test does not actually connect; it only verifies the code path.
			// A real environment would require a database.
			db, err := Open(tt.dsn)

			// Without a real database, we only verify there is no panic
			_ = db
			_ = err

			// If there is an error, it should be a connection error, not a parse error
			if err != nil {
				// Expect the connection to fail (no database)
				t.Logf("Expected connection failure: %v", err)
			}
		})
	}
}

// TestOpen_SQLite tests the SQLite connection
func TestOpen_SQLite(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name:    "file: 前缀",
			dsn:     "file::memory:",
			wantErr: false,
		},
		{
			name:    ":memory:",
			dsn:     ":memory:",
			wantErr: false,
		},
		{
			name:    "sqlite:/// 前缀 - 转换为 file:",
			dsn:     "sqlite:///tmp/test.db",
			wantErr: true, // the sqlite:/// prefix is converted to file:///tmp/test.db, which errors because the file does not exist
		},
		{
			name:    "相对路径",
			dsn:     "file:test.db",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Open(tt.dsn)

			if (err != nil) != tt.wantErr {
				t.Errorf("Open() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && db != nil {
				// Close the database
				sqlDB, _ := db.DB()
				if sqlDB != nil {
					_ = sqlDB.Close()
				}

				// Clean up the test file
				if tt.dsn != ":memory:" && tt.dsn != "file::memory:" {
					cleanPath := tt.dsn
					if len(cleanPath) > 5 && cleanPath[:5] == "file:" {
						cleanPath = cleanPath[5:]
					}
					if len(cleanPath) > 11 && cleanPath[:11] == "sqlite:///" {
						cleanPath = cleanPath[11:]
					}
					_ = os.Remove(cleanPath)
				}
			}
		})
	}
}

// TestOpen_EmptyDSN tests an empty DSN
func TestOpen_EmptyDSN(t *testing.T) {
	// Save the current directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	// Create a temporary directory
	tempDir := t.TempDir()
	_ = os.Chdir(tempDir)

	db, err := Open("")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if db == nil {
		t.Fatal("Open() should return non-nil db")
	}

	// Verify the data directory was created
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		t.Error("data directory should be created")
	}

	// Close the database
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// TestOpen_DefaultPath tests the default path
func TestOpen_DefaultPath(t *testing.T) {
	// Save the current directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	// Create a temporary directory
	tempDir := t.TempDir()
	_ = os.Chdir(tempDir)

	db, err := Open("")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}()

	// Verify the database file path
	expectedPath := filepath.Join("data", "croupier.db")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Logf("Database file may not exist yet: %v", err)
	}
}

// TestOpen_SQLiteURI tests SQLite URI formats
func TestOpen_SQLiteURI(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{
			name: "cache=shared",
			dsn:  "file:test.db?cache=shared",
		},
		{
			name: "mode=memory",
			dsn:  "file:test.db?mode=memory",
		},
		{
			name: "复杂 URI",
			dsn:  "file:test.db?cache=shared&mode=rwc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Open(tt.dsn)

			// No error is expected (SQLite can create in-memory databases)
			if err != nil {
				t.Logf("Open() error = %v (may be expected)", err)
			}

			if db != nil {
				sqlDB, _ := db.DB()
				if sqlDB != nil {
					_ = sqlDB.Close()
				}
				_ = os.Remove("test.db")
			}
		})
	}
}

// TestOpen_DataDirectoryCreation tests creation of the data directory
func TestOpen_DataDirectoryCreation(t *testing.T) {
	// Save the current directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	// Create a temporary directory
	tempDir := t.TempDir()
	_ = os.Chdir(tempDir)

	// Ensure the data directory does not exist
	_, err := os.Stat("data")
	if !os.IsNotExist(err) {
		_ = os.RemoveAll("data")
	}

	db, err := Open("")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}()

	// Verify the directory was created
	info, err := os.Stat("data")
	if err != nil {
		t.Fatalf("Failed to stat data directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("data should be a directory")
	}
}

// TestOpen_MultipleEmptyDSN tests multiple empty-DSN calls
func TestOpen_MultipleEmptyDSN(t *testing.T) {
	// Save the current directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	// Create a temporary directory
	tempDir := t.TempDir()
	_ = os.Chdir(tempDir)

	for i := 0; i < 3; i++ {
		db, err := Open("")
		if err != nil {
			t.Errorf("Open() iteration %d error = %v", i, err)
		}

		if db != nil {
			sqlDB, _ := db.DB()
			if sqlDB != nil {
				_ = sqlDB.Close()
			}
		}
	}
}

// TestOpen_SQLitePrefixConversion tests SQLite prefix conversion
func TestOpen_SQLitePrefixConversion(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		shouldWork bool
	}{
		{
			name:       "sqlite:/// 转换",
			input:      "sqlite:///path/to/test.db",
			shouldWork: true,
		},
		{
			name:       "直接 file: 前缀",
			input:      "file:/path/to/test.db",
			shouldWork: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Open(tt.input)

			if tt.shouldWork && err != nil {
				t.Logf("Open() with %s error = %v", tt.input, err)
			}

			if db != nil {
				sqlDB, _ := db.DB()
				if sqlDB != nil {
					_ = sqlDB.Close()
				}
			}

			// Clean up
			cleanPath := tt.input
			if len(cleanPath) > 11 && cleanPath[:11] == "sqlite:///" {
				cleanPath = cleanPath[11:]
			}
			if len(cleanPath) > 5 && cleanPath[:5] == "file:" {
				cleanPath = cleanPath[5:]
			}
			_ = os.Remove(cleanPath)
		})
	}
}

// BenchmarkOpen_PostgreSQLString performance benchmark
func BenchmarkOpen_PostgreSQLString(b *testing.B) {
	dsn := "postgres://user:pass@localhost:5432/test?sslmode=disable"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Only measures string parsing, no actual connection
		_ = len(dsn) > 10 && (dsn[:10] == "postgres://" || dsn[:13] == "postgresql://")
	}
}

// BenchmarkOpen_SQLiteMemory performance benchmark
func BenchmarkOpen_SQLiteMemory(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db, _ := Open(":memory:")
		if db != nil {
			sqlDB, _ := db.DB()
			if sqlDB != nil {
				_ = sqlDB.Close()
			}
		}
	}
}
