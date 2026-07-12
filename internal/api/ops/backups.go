package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// Backup operations sub-service

type BackupService struct {
	svcCtx *svc.ServiceContext
}

func NewBackupService(svcCtx *svc.ServiceContext) *BackupService {
	return &BackupService{svcCtx: svcCtx}
}

func (s *BackupService) List(ctx context.Context, gameId, env string) ([]Backup, error) {
	if s.svcCtx.BackupModel == nil {
		return nil, errors.New("backup model unavailable")
	}

	opts := model.ListBackupsOptions{
		PaginationOptions: model.NewPagination(1, 1000),
	}
	backups, _, err := s.svcCtx.BackupModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]Backup, 0, len(backups))
	for _, b := range backups {
		items = append(items, Backup{
			Id:        fmt.Sprintf("%d", b.ID),
			Name:      b.Name,
			Type:      b.Type,
			Status:    b.Status,
			Size:      b.Size,
			CreatedAt: utils.FormatTimestamp(b.CreatedAt),
		})
	}

	return items, nil
}

func (s *BackupService) Create(ctx context.Context, gameId, env, backupType string, timeout int) (string, error) {
	if s.svcCtx.BackupModel == nil {
		return "", errors.New("backup model unavailable")
	}

	backup := &model.Backup{
		BackupID: uuid.New().String(),
		Name:     fmt.Sprintf("%s-%s-backup", gameId, env),
		Type:     backupType,
		Status:   "pending",
	}
	if err := s.svcCtx.BackupModel.Create(ctx, backup); err != nil {
		return "", err
	}

	return backup.BackupID, nil
}

func (s *BackupService) Delete(ctx context.Context, backupId string) error {
	if s.svcCtx.BackupModel == nil {
		return errors.New("backup model unavailable")
	}

	// Find by backup_id first
	backup, err := s.svcCtx.BackupModel.FindByBackupID(ctx, backupId)
	if err != nil {
		return err
	}
	return s.svcCtx.BackupModel.Delete(ctx, backup.ID)
}

func (s *BackupService) GetDownloadURL(ctx context.Context, backupId string) (string, string, error) {
	if s.svcCtx.BackupModel == nil {
		return "", "", errors.New("backup model unavailable")
	}

	backup, err := s.svcCtx.BackupModel.FindByBackupID(ctx, backupId)
	if err != nil {
		return "", "", err
	}

	// Prefer the recorded storage location (file path, file:// URI, or remote URL).
	// Fall back to the legacy placeholder route so callers that haven't populated
	// Location still get a stable, predictable URL.
	url := strings.TrimSpace(backup.Location)
	if url == "" {
		url = fmt.Sprintf("/backups/%s/download", backupId)
	}
	return url, utils.FormatTimestamp(time.Now().Add(24 * time.Hour)), nil
}
