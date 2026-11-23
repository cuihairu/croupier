package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/backupexec"
	"github.com/cuihairu/croupier/internal/svc"

	"gorm.io/gorm"
)

const officialBackupAdvancedID = "official.backup-advanced"
const backupRecordsKey = "backups"

// backupDedupeWindow 是无客户端幂等键时的同名同类型去重时间窗（AUD-053）：
// 双击/快速重试兜底；正常运维节奏（同名定时备份间隔 ≥ 窗口）不受影响。
const backupDedupeWindow = 10 * time.Minute

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a paginated list of backups
func (s *Service) List(ctx context.Context, req *BackupsListRequest) (*BackupsListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看备份", "admin:all", "backup:read"); err != nil {
		return nil, err
	}
	if s.svcCtx == nil || s.svcCtx.BackupModel == nil {
		return nil, errors.New("备份模型未初始化")
	}
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
		PaginationOptions: model.NewPagination(req.Page, req.PageSize),
		Type:              strings.TrimSpace(req.Type),
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
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权创建备份", "admin:all", "backup:create"); err != nil {
		return nil, err
	}
	if s.svcCtx == nil || s.svcCtx.BackupModel == nil {
		return nil, errors.New("备份模型未初始化")
	}
	backupType := strings.ToLower(strings.TrimSpace(req.Type))
	if backupType == "" {
		backupType = "full"
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("%s-%s", backupType, time.Now().UTC().Format("20060102-150405"))
	}

	// AUD-053 创建幂等：同键活跃（pending/succeeded）备份直接复用——重试/
	// 双击不再叠加重备份；failed 不算活跃（失败后同键重试建新备份）。
	// 无客户端键时按 name+type 时间窗兜底（自动生成名含秒级时间戳，同秒
	// 双击会撞出同名，同样被窗口收住）。
	idemKey := strings.TrimSpace(req.IdempotencyKey)
	if idemKey != "" {
		if existing, err := s.svcCtx.BackupModel.FindActiveByIdempotencyKey(ctx, idemKey); err == nil && existing != nil {
			return &BackupCreateResponse{Backup: buildBackupDTO(existing)}, nil
		}
		// 同键 failed 行让位：失败后重试建新行，唯一索引不得挡路
		//（预检未命中 = 无活跃行；可能存在 failed 行占着键）。
		if err := s.svcCtx.BackupModel.ReleaseIdempotencyKey(ctx, idemKey); err != nil {
			return nil, err
		}
	} else if existing, err := s.svcCtx.BackupModel.FindRecentDuplicateByNameType(ctx, name, backupType, backupDedupeWindow); err == nil && existing != nil {
		return &BackupCreateResponse{Backup: buildBackupDTO(existing)}, nil
	}

	backup := &model.Backup{
		BackupID: utils.GenerateBackupID(),
		Name:     name,
		Type:     backupType,
		Status:   "pending",
	}
	if idemKey != "" {
		backup.IdempotencyKey = &idemKey
	}

	if err := s.svcCtx.BackupModel.Create(ctx, backup); err != nil {
		// 并发双写竞争：预检双双落空后同键插入，唯一索引兜底——后到方
		// 回落到已存在行，对外仍是一次成功的幂等创建。
		if idemKey != "" && isIdempotencyConflict(err) {
			if existing, ferr := s.svcCtx.BackupModel.FindActiveByIdempotencyKey(ctx, idemKey); ferr == nil && existing != nil {
				return &BackupCreateResponse{Backup: buildBackupDTO(existing)}, nil
			}
		}
		return nil, err
	}
	_ = s.upsertBackupToExtension(ctx, buildBackupDTO(backup))
	_ = s.recordBackupEvent(ctx, "backups_create", "backup created",
		fmt.Sprintf(`{"backup_id":"%s","type":"%s"}`, backup.BackupID, backup.Type),
	)

	// 真实执行（异步）：导出 → 校验 → 上传对象存储 → 更新记录。
	// 失败置 failed 并保留错误信息；立即响应 pending 状态给调用方。
	if exec := s.executor(); exec != nil {
		backupID, name, backupType := backup.BackupID, backup.Name, backup.Type
		go func() {
			// WithoutCancel 从入参派生：备份异步执行与请求生命周期解绑
			// （客户端断开不中断），保留 ctx 值（game DB 路由等）（CR-FIX-22）。
			bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Minute)
			defer cancel()
			if err := exec.RunBackup(bg, backupID, name, backupType); err != nil {
				_ = s.recordBackupEvent(bg, "backups_run_failed",
					"backup execution failed",
					fmt.Sprintf(`{"backup_id":"%s","error":%q}`, backupID, err.Error()))
			} else {
				_ = s.recordBackupEvent(bg, "backups_run_succeeded",
					"backup execution succeeded",
					fmt.Sprintf(`{"backup_id":"%s"}`, backupID))
			}
		}()
	}

	return &BackupCreateResponse{
		Backup: buildBackupDTO(backup),
	}, nil
}

// isIdempotencyConflict 识别幂等键唯一索引冲突（AUD-053）：三方言的错误
// 文案都带列名（sqlite "UNIQUE constraint failed: backups.idempotency_key"、
// postgres "duplicate key value violates unique constraint ...idempotency_key"、
// mysql "Duplicate entry ... for key 'idempotency_key'"）。
func isIdempotencyConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "idempotency_key")
}

// executor 构造真实备份执行器（driver 不支持或存储未就绪时返回 nil，
// Create 退化为纯记录模式，与旧行为兼容）。
func (s *Service) executor() *backupexec.Executor {
	if s == nil || s.svcCtx == nil || s.svcCtx.BackupModel == nil || s.svcCtx.ObjectStore == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(s.svcCtx.Config.Database.Driver)) {
	case "mysql", "postgres", "pg", "sqlite":
		return backupexec.New(
			s.svcCtx.Config.Database.Driver,
			s.svcCtx.Config.Database.DataSource,
			s.svcCtx.BackupModel,
			s.svcCtx.ObjectStore,
			"backups",
		)
	default:
		return nil
	}
}

// Delete deletes a backup
func (s *Service) Delete(ctx context.Context, req *BackupDeleteRequest) error {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权删除备份", "admin:all", "backup:delete"); err != nil {
		return err
	}
	if s.svcCtx == nil || s.svcCtx.BackupModel == nil {
		return errors.New("备份模型未初始化")
	}
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
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权下载备份", "admin:all", "backup:read"); err != nil {
		return nil, err
	}
	if s.svcCtx == nil || s.svcCtx.BackupModel == nil {
		return nil, errors.New("备份模型未初始化")
	}
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

	// AUD-COV-001 P1：其余形态一律按对象 key 经 objstore 读取。原实现按
	// 记录 location 接受 file:// 与任意绝对/相对路径直读本地文件系统，可
	// 被诱导读取服务器任意文件；objstore 驱动自带 base 目录内校验（file
	// 驱动 validateAndCleanPath），与备份执行器写入/恢复语义同源。
	if s.svcCtx.ObjectStore == nil {
		return nil, errorx.NewInternalError("对象存储未就绪")
	}
	size, err := s.objectSize(ctx, location)
	if err != nil {
		return nil, errorx.NewNotFound("备份对象不存在: " + backupID)
	}
	reader, err := s.svcCtx.ObjectStore.Get(ctx, location)
	if err != nil {
		return nil, errorx.NewInternalError("无法打开备份文件")
	}

	return &DownloadPayload{
		Filename: utils.GuessBackupFilename(record),
		Size:     size,
		Reader:   reader,
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

// objectSize 经 List 前缀查询精确匹配取对象大小（Store 接口无 Stat）。前缀取
// key 的父目录而非完整 key：file 驱动的 List 以目录为根做 walk，完整 key 会
// 命中「跳过根目录本身」拿不到条目；对象存储驱动按前缀列出同目录对象。根级
// key（无 "/"）用空前缀列全量，按 marker 翻页兜底。
func (s *Service) objectSize(ctx context.Context, key string) (int64, error) {
	prefix := ""
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		prefix = key[:idx+1]
	}
	marker := ""
	for page := 0; page < 20; page++ {
		res, err := s.svcCtx.ObjectStore.List(ctx, prefix, marker, "", 1000)
		if err != nil {
			return 0, err
		}
		for _, obj := range res.Objects {
			if obj.Key == key {
				return obj.Size, nil
			}
		}
		if !res.IsTruncated || res.NextMarker == "" {
			break
		}
		marker = res.NextMarker
	}
	return 0, errors.New("object not found")
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
	if len(bytes.TrimSpace(item.ConfigJSON)) > 0 {
		if err := json.Unmarshal(item.ConfigJSON, &config); err != nil {
			return nil, false, err
		}
	}
	raw, exists := config[backupRecordsKey]
	if !exists || raw == nil {
		return nil, false, nil
	}
	// raw 来自 json.Unmarshal(map[string]any) 的取值，值类型仅可能为 JSON
	// 基础类型（nil/bool/float64/string/[]any/map[string]any），再 Marshal 恒
	// 成功，error 分支不可达，已删除。
	data, _ := json.Marshal(raw)
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
	if len(bytes.TrimSpace(item.ConfigJSON)) > 0 {
		_ = json.Unmarshal(item.ConfigJSON, &config)
	}
	config[backupRecordsKey] = items
	secretRefs := map[string]string{}
	if len(bytes.TrimSpace(item.SecretRefsJSON)) > 0 {
		_ = json.Unmarshal(item.SecretRefsJSON, &secretRefs)
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
