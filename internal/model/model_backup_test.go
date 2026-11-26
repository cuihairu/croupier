package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewBackupModel(t *testing.T) {
	db := setupBackupTestDB(t)
	model := NewBackupModel(db)
	assert.NotNil(t, model)
}

func TestBackupModel_Create(t *testing.T) {
	db := setupBackupTestDB(t)
	model := NewBackupModel(db)
	ctx := context.Background()

	backup := &Backup{
		BackupID: "backup-001",
		Name:     "Daily Backup",
		Size:     1024000,
		Type:     "full",
		Status:   "completed",
		Location: "/backups/daily.tar.gz",
		Checksum: "abc123",
	}

	err := model.Create(ctx, backup)
	require.NoError(t, err)
	assert.NotZero(t, backup.ID)
}

func TestBackupModel_FindByID(t *testing.T) {
	db := setupBackupTestDB(t)
	model := NewBackupModel(db)
	ctx := context.Background()

	backup := &Backup{
		BackupID: "findbyid-001",
		Name:     "Find By ID Test",
		Size:     512000,
		Type:     "incremental",
		Status:   "completed",
		Location: "/backups/inc.tar.gz",
	}
	err := model.Create(ctx, backup)
	require.NoError(t, err)

	// Find existing
	found, err := model.FindByID(ctx, backup.ID)
	require.NoError(t, err)
	assert.Equal(t, "Find By ID Test", found.Name)

	// Find non-existent
	_, err = model.FindByID(ctx, 99999)
	assert.Error(t, err)
}

func TestBackupModel_FindByBackupID(t *testing.T) {
	db := setupBackupTestDB(t)
	model := NewBackupModel(db)
	ctx := context.Background()

	backup := &Backup{
		BackupID: "findbybkid-001",
		Name:     "Find By Backup ID Test",
		Size:     256000,
		Type:     "full",
		Status:   "completed",
		Location: "/backups/full.tar.gz",
	}
	err := model.Create(ctx, backup)
	require.NoError(t, err)

	// Find by backup ID
	found, err := model.FindByBackupID(ctx, "findbybkid-001")
	require.NoError(t, err)
	assert.Equal(t, "Find By Backup ID Test", found.Name)

	// Find non-existent
	_, err = model.FindByBackupID(ctx, "non-existent")
	assert.Error(t, err)

	// Empty backup ID
	_, err = model.FindByBackupID(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup id is required")

	_, err = model.FindByBackupID(ctx, "   ")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup id is required")
}

func TestBackupModel_Delete(t *testing.T) {
	db := setupBackupTestDB(t)
	model := NewBackupModel(db)
	ctx := context.Background()

	backup := &Backup{
		BackupID: "delete-001",
		Name:     "Delete Test",
		Size:     128000,
		Type:     "full",
		Status:   "completed",
		Location: "/backups/delete.tar.gz",
	}
	err := model.Create(ctx, backup)
	require.NoError(t, err)

	// Delete
	err = model.Delete(ctx, backup.ID)
	require.NoError(t, err)

	// Verify
	_, err = model.FindByID(ctx, backup.ID)
	assert.Error(t, err)
}

func TestBackupModel_DeleteByBackupID(t *testing.T) {
	db := setupBackupTestDB(t)
	model := NewBackupModel(db)
	ctx := context.Background()

	backup := &Backup{
		BackupID: "deletebybkid-001",
		Name:     "Delete By Backup ID Test",
		Size:     64000,
		Type:     "full",
		Status:   "completed",
		Location: "/backups/delbkid.tar.gz",
	}
	err := model.Create(ctx, backup)
	require.NoError(t, err)

	// Delete by backup ID
	err = model.DeleteByBackupID(ctx, "deletebybkid-001")
	require.NoError(t, err)

	// Verify
	_, err = model.FindByBackupID(ctx, "deletebybkid-001")
	assert.Error(t, err)

	// Empty backup ID
	err = model.DeleteByBackupID(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup id is required")
}

func TestBackupModel_List(t *testing.T) {
	db := setupBackupTestDB(t)
	model := NewBackupModel(db)
	ctx := context.Background()

	backups := []*Backup{
		{BackupID: "list-001", Name: "Backup 1", Size: 1000, Type: "full", Status: "completed", Location: "/b1.tar.gz"},
		{BackupID: "list-002", Name: "Backup 2", Size: 2000, Type: "incremental", Status: "completed", Location: "/b2.tar.gz"},
		{BackupID: "list-003", Name: "Backup 3", Size: 3000, Type: "full", Status: "pending", Location: "/b3.tar.gz"},
	}
	for _, b := range backups {
		err := model.Create(ctx, b)
		require.NoError(t, err)
	}

	// List all（首次声明即消费 total；items 在分页断言处才需要）
	var items []Backup
	_, total, err := model.List(ctx, ListBackupsOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))

	// Filter by type
	_, total, err = model.List(ctx, ListBackupsOptions{Type: "full"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))

	// Test pagination
	items, _, err = model.List(ctx, ListBackupsOptions{PaginationOptions: PaginationOptions{Page: 1, PageSize: 2}})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(items), 2)
}

// ===== CertificateModel Tests =====
