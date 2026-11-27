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

// AUD-083: the mirror-only filter/pagination/normalization helpers were removed
// with the mirror-priority list; paging/filtering is covered through
// TestService_List_FromDatabase against BackupModel.List (model.NewPagination).

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
