package backup

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_List_NilModel(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.List(context.Background(), &BackupsListRequest{})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Create_NilModel(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	resp, err := service.Create(context.Background(), &BackupCreateRequest{})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Delete_NilModel(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	err := service.Delete(context.Background(), &BackupDeleteRequest{ID: "1"})
	assert.Error(t, err)
}

func TestService_Download_NilModel(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	_, err := service.Download(context.Background(), &BackupDownloadRequest{ID: "1"})
	assert.Error(t, err)
}

func TestNormalizedPage(t *testing.T) {
	assert.Equal(t, 1, normalizedPage(0))
	assert.Equal(t, 1, normalizedPage(-1))
	assert.Equal(t, 1, normalizedPage(1))
	assert.Equal(t, 10, normalizedPage(10))
}

func TestNormalizedPageSize(t *testing.T) {
	assert.Equal(t, 20, normalizedPageSize(0))
	assert.Equal(t, 20, normalizedPageSize(-1))
	assert.Equal(t, 1000, normalizedPageSize(1000)) // no upper clamp by design
	assert.Equal(t, 10, normalizedPageSize(10))
}

func TestFilterBackupsByType(t *testing.T) {
	backups := []Backup{
		{Type: "database"},
		{Type: "config"},
		{Type: "database"},
	}
	filtered := filterBackupsByType(backups, "database")
	assert.Len(t, filtered, 2)
	filtered = filterBackupsByType(backups, "")
	assert.Len(t, filtered, 3)
}

func TestBuildBackupDTO(t *testing.T) {
	record := &model.Backup{
		BackupID: "test-123",
		Name:     "test-backup",
		Type:     "full",
		Status:   "completed",
		Size:     1024,
	}
	dto := buildBackupDTO(record)
	assert.Equal(t, "test-123", dto.Id)
	assert.Equal(t, "test-backup", dto.Name)
	assert.Equal(t, "full", dto.Type)
	assert.Equal(t, "completed", dto.Status)
	assert.Equal(t, int64(1024), dto.Size)
}

func TestBuildBackupList(t *testing.T) {
	backups := []model.Backup{
		{BackupID: "1", Name: "backup1"},
		{BackupID: "2", Name: "backup2"},
	}
	list := buildBackupList(backups)
	require.Len(t, list, 2)
	assert.Equal(t, "1", list[0].Id)
	assert.Equal(t, "2", list[1].Id)
}

func TestPaginateBackups(t *testing.T) {
	backups := make([]Backup, 25)
	for i := range backups {
		backups[i] = Backup{Id: string(rune('A' + i))}
	}
	paged, total := paginateBackups(backups, 1, 10)
	assert.Len(t, paged, 10)
	assert.Equal(t, 25, total)

	paged, total = paginateBackups(backups, 3, 10)
	assert.Len(t, paged, 5)
	assert.Equal(t, 25, total)

	paged, total = paginateBackups(backups, 10, 10)
	assert.Len(t, paged, 0)
	assert.Equal(t, 25, total)
}

func TestTryRemoteDownload(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	// Remote URL
	payload, ok := service.tryRemoteDownload("https://example.com/backup.zip")
	assert.True(t, ok)
	assert.Equal(t, "https://example.com/backup.zip", payload.RedirectURL)

	// File URL
	_, ok = service.tryRemoteDownload("file:///tmp/backup.zip")
	assert.False(t, ok)

	// Relative path
	_, ok = service.tryRemoteDownload("backup.zip")
	assert.False(t, ok)
}

func TestResolveBackupPath(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	// Absolute path
	path, err := service.resolveBackupPath("/tmp/backup.zip")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/backup.zip", path)

	// File URI
	path, err = service.resolveBackupPath("file:///tmp/backup.zip")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/backup.zip", path)
}
