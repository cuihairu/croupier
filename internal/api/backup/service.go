package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"

	"gorm.io/gorm"
)

const officialBackupAdvancedID = "official.backup-advanced"
const backupRecordsKey = "backups"

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a paginated list of backups
func (s *Service) List(ctx context.Context, req *BackupsListRequest) (*BackupsListResponse, error) {
	if req == nil {
		req = &BackupsListRequest{}
	}
	if items, ok, err := s.loadBackupsFromExtensionInstallation(ctx); err != nil {
		return nil, err
	} else if ok {
		filtered := filterBackupsByType(items, strings.TrimSpace(req.Type))
		paged, total := paginateBackups(filtered, req.Page, req.PageSize)
		return &BackupsListResponse{
			Items: paged,
			Total: int64(total),
			Page:  normalizedPage(req.Page),
			Size:  normalizedPageSize(req.PageSize),
		}, nil
	}
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
	_ = s.upsertBackupToExtension(ctx, buildBackupDTO(backup))
	_ = s.recordBackupEvent(ctx, "backups_create", "backup created",
		fmt.Sprintf(`{"backup_id":"%s","type":"%s"}`, backup.BackupID, backup.Type),
	)

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
	if err := s.svcCtx.BackupModel.DeleteByBackupID(ctx, backupID); err != nil {
		return err
	}
	_ = s.removeBackupFromExtension(ctx, backupID)
	_ = s.recordBackupEvent(ctx, "backups_delete", "backup deleted",
		fmt.Sprintf(`{"backup_id":"%s"}`, backupID),
	)
	return nil
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

func (s *Service) findActiveBackupInstallation(ctx context.Context) (*model.ExtensionInstallation, bool, error) {
	if s == nil || s.svcCtx == nil || s.svcCtx.Extensions == nil || s.svcCtx.Extensions.Installation == nil {
		return nil, false, nil
	}
	items, _, err := s.svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		ExtensionID: officialBackupAdvancedID,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		return nil, false, err
	}
	for i := range items {
		item := items[i]
		if strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") ||
			strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
			continue
		}
		return &item, true, nil
	}
	return nil, false, nil
}

func (s *Service) recordBackupEvent(ctx context.Context, eventType, message, payload string) error {
	item, ok, err := s.findActiveBackupInstallation(ctx)
	if err != nil || !ok || item == nil {
		return err
	}
	operator := "system"
	if username, userErr := utils.CurrentUsername(ctx); userErr == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return s.svcCtx.Extensions.Installation.RecordEvent(ctx, item.ID, eventType, "info", message, operator, payload)
}

func (s *Service) loadBackupsFromExtensionInstallation(ctx context.Context) ([]Backup, bool, error) {
	item, ok, err := s.findActiveBackupInstallation(ctx)
	if err != nil || !ok || item == nil {
		return nil, false, err
	}
	config := map[string]any{}
	if strings.TrimSpace(item.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(item.ConfigJSON), &config); err != nil {
			return nil, false, err
		}
	}
	raw, exists := config[backupRecordsKey]
	if !exists || raw == nil {
		return nil, false, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, false, err
	}
	items := []Backup{}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, false, err
	}
	return items, true, nil
}

func (s *Service) saveBackupsToExtensionInstallation(ctx context.Context, items []Backup) error {
	item, ok, err := s.findActiveBackupInstallation(ctx)
	if err != nil || !ok || item == nil {
		return err
	}
	config := map[string]any{}
	if strings.TrimSpace(item.ConfigJSON) != "" {
		_ = json.Unmarshal([]byte(item.ConfigJSON), &config)
	}
	config[backupRecordsKey] = items
	secretRefs := map[string]string{}
	if strings.TrimSpace(item.SecretRefsJSON) != "" {
		_ = json.Unmarshal([]byte(item.SecretRefsJSON), &secretRefs)
	}
	operator := "system"
	if username, userErr := utils.CurrentUsername(ctx); userErr == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return s.svcCtx.Extensions.Installation.UpdateConfig(ctx, item.ID, config, secretRefs, operator)
}

func (s *Service) upsertBackupToExtension(ctx context.Context, current Backup) error {
	if strings.TrimSpace(current.Id) == "" {
		return nil
	}
	items, _, err := s.loadBackupsFromExtensionInstallation(ctx)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(current.Id)
	for i := range items {
		if strings.TrimSpace(items[i].Id) == id {
			items[i] = current
			return s.saveBackupsToExtensionInstallation(ctx, items)
		}
	}
	items = append(items, current)
	return s.saveBackupsToExtensionInstallation(ctx, items)
}

func (s *Service) removeBackupFromExtension(ctx context.Context, backupID string) error {
	id := strings.TrimSpace(backupID)
	if id == "" {
		return nil
	}
	items, ok, err := s.loadBackupsFromExtensionInstallation(ctx)
	if err != nil {
		return err
	}
	if !ok || len(items) == 0 {
		return nil
	}
	filtered := make([]Backup, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Id) == id {
			continue
		}
		filtered = append(filtered, item)
	}
	return s.saveBackupsToExtensionInstallation(ctx, filtered)
}

func filterBackupsByType(items []Backup, backupType string) []Backup {
	if backupType == "" {
		return items
	}
	out := make([]Backup, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Type), backupType) {
			out = append(out, item)
		}
	}
	return out
}

func paginateBackups(items []Backup, page, size int) ([]Backup, int) {
	total := len(items)
	page = normalizedPage(page)
	size = normalizedPageSize(size)
	start := (page - 1) * size
	if start >= total {
		return []Backup{}, total
	}
	end := start + size
	if end > total {
		end = total
	}
	return items[start:end], total
}

func normalizedPage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizedPageSize(size int) int {
	if size <= 0 {
		return 20
	}
	return size
}
