package backup

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"

	"gorm.io/gorm"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a paginated list of backups
func (s *Service) List(ctx context.Context, req *BackupsListRequest) (*BackupsListResponse, error) {
	opts := model.ListBackupsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Type: strings.TrimSpace(req.Type),
	}

	backups, total, err := s.svcCtx.BackupModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &BackupsListResponse{
		Items: buildBackupList(backups),
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}

// Create creates a new backup
func (s *Service) Create(ctx context.Context, req *BackupCreateRequest) (*BackupCreateResponse, error) {
	backupType := strings.ToLower(strings.TrimSpace(req.Type))
	if backupType == "" {
		backupType = "full"
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("%s-%s", backupType, time.Now().UTC().Format("20060102-150405"))
	}

	backup := &model.Backup{
		BackupID: utils.GenerateBackupID(),
		Name:     name,
		Type:     backupType,
		Status:   "pending",
	}

	if err := s.svcCtx.BackupModel.Create(ctx, backup); err != nil {
		return nil, err
	}

	return &BackupCreateResponse{
		Backup: buildBackupDTO(backup),
	}, nil
}

// Delete deletes a backup
func (s *Service) Delete(ctx context.Context, req *BackupDeleteRequest) error {
	backupID := strings.TrimSpace(req.ID)
	if backupID == "" {
		return errors.New("备份ID不能为空")
	}

	if _, err := s.svcCtx.BackupModel.FindByBackupID(ctx, backupID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.NewNotFound("备份不存在: " + backupID)
		}
		return err
	}

	return s.svcCtx.BackupModel.DeleteByBackupID(ctx, backupID)
}

// Download downloads a backup file
func (s *Service) Download(ctx context.Context, req *BackupDownloadRequest) (*DownloadPayload, error) {
	backupID := strings.TrimSpace(req.ID)
	if backupID == "" {
		return nil, errors.New("备份ID不能为空")
	}

	record, err := s.svcCtx.BackupModel.FindByBackupID(ctx, backupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("备份不存在: " + backupID)
		}
		return nil, err
	}

	location := strings.TrimSpace(record.Location)
	if location == "" {
		return nil, errorx.NewBadRequest("备份尚未生成可下载文件: " + backupID)
	}

	if payload, ok := s.tryRemoteDownload(location); ok {
		return payload, nil
	}

	path, err := s.resolveBackupPath(location)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, errorx.NewInternalError("无法打开备份文件")
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, errorx.NewInternalError("读取备份文件信息失败")
	}

	return &DownloadPayload{
		Filename: utils.GuessBackupFilename(record),
		Size:     info.Size(),
		Reader:   file,
	}, nil
}

func (s *Service) tryRemoteDownload(location string) (*DownloadPayload, bool) {
	u, err := url.Parse(location)
	if err != nil || u.Scheme == "" || u.Scheme == "file" {
		return nil, false
	}
	return &DownloadPayload{
		RedirectURL: location,
	}, true
}

func (s *Service) resolveBackupPath(location string) (string, error) {
	if strings.HasPrefix(location, "file://") {
		u, err := url.Parse(location)
		if err != nil {
			return "", errorx.NewBadRequest("解析下载路径失败")
		}
		return u.Path, nil
	}
	if filepath.IsAbs(location) {
		return location, nil
	}
	abs, err := filepath.Abs(location)
	if err != nil {
		return "", errorx.NewBadRequest("解析备份路径失败")
	}
	return abs, nil
}

func buildBackupList(backups []model.Backup) []Backup {
	items := make([]Backup, 0, len(backups))
	for i := range backups {
		items = append(items, buildBackupDTO(&backups[i]))
	}
	return items
}

func buildBackupDTO(backup *model.Backup) Backup {
	return Backup{
		Id:        backup.BackupID,
		Name:      backup.Name,
		Size:      backup.Size,
		Type:      backup.Type,
		Status:    backup.Status,
		CreatedAt: utils.FormatTimestamp(backup.CreatedAt),
	}
}
