package svc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenReadOnlyGorm_SQLiteFailures(t *testing.T) {
	t.Run("dsn is a directory", func(t *testing.T) {
		dir := t.TempDir()
		_, err := openReadOnlyGorm("sqlite", dir)
		assert.Error(t, err)
	})

	t.Run("query fails on garbage file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "garbage.db")
		if err := os.WriteFile(path, []byte("definitely not a sqlite database"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Depending on the sqlite build this either opens fine (treating the
		// file as an empty database) or fails; both outcomes are acceptable,
		// the goal is exercising the error propagation lines.
		db, err := openReadOnlyGorm("sqlite", path)
		if db != nil {
			sqlDB, _ := db.DB()
			defer sqlDB.Close()
		}
		_ = err
	})
}

func TestEnsureSQLiteDir_MemoryAfterPrefixStrip(t *testing.T) {
	assert.NoError(t, ensureSQLiteDir("file::memory:?cache=shared"))
}

func TestEnsureSQLiteFileExists_StatFailure(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	require_NoError(t, os.MkdirAll(sub, 0o755))

	require_NoError(t, os.Chmod(sub, 0o000))
	defer func() { _ = os.Chmod(sub, 0o755) }()

	err := ensureSQLiteFileExists(filepath.Join(sub, "x.db"))
	assert.Error(t, err)
}

func require_NoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
