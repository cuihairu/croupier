package ops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/backupexec"
	"github.com/cuihairu/croupier/internal/svc"
)

// Backup operations sub-service

type BackupService struct {
	svcCtx *svc.ServiceContext
}

func NewBackupService(svcCtx *svc.ServiceContext) *BackupService {
	return &BackupService{svcCtx: svcCtx}
}

// defaultBackupTimeout bounds one async backup run when the caller gives no
// explicit timeout (matches the platform backup service's 30m bound).
const defaultBackupTimeout = 30 * time.Minute

// backupTimeout resolves the async run budget. timeout is in seconds; <=0 falls
// back to the default.
func backupTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultBackupTimeout
	}
	return time.Duration(seconds) * time.Second
}

func (s *BackupService) List(ctx context.Context, gameId, env string) ([]Backup, error) {
	if s.svcCtx.BackupModel == nil {
		return nil, errors.New("backup model unavailable")
	}

	opts := model.ListBackupsOptions{
		PaginationOptions: model.NewPagination(1, 1000),
		GameID:            strings.TrimSpace(gameId),
		Env:               strings.TrimSpace(env),
	}
	backups, _, err := s.svcCtx.BackupModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return buildOpsBackupItems(backups), nil
}

func (s *BackupService) Create(ctx context.Context, gameId, env, backupType string, timeout int) (string, error) {
	if s.svcCtx == nil {
		return "", errors.New("backup model unavailable")
	}
	backup, err := createBackupForScope(ctx, s.svcCtx,
		svc.GameScope{GameID: strings.TrimSpace(gameId), Env: strings.TrimSpace(env)},
		"", backupType, timeout)
	if err != nil {
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

// Verify 重算备份对象 sha256 并与记录校验值比对（OPS-008 校验读路径）。
// 返回 matched 表示校验一致；读取/哈希失败返回错误。
func (s *BackupService) Verify(ctx context.Context, backupId string) (bool, error) {
	if s.svcCtx == nil || s.svcCtx.BackupModel == nil {
		return false, errors.New("backup model unavailable")
	}
	if s.svcCtx.ObjectStore == nil {
		return false, errors.New("object store unavailable")
	}
	backup, err := s.svcCtx.BackupModel.FindByBackupID(ctx, strings.TrimSpace(backupId))
	if err != nil {
		return false, err
	}
	return backupexec.VerifyChecksum(ctx, s.svcCtx.ObjectStore, backup)
}

// createBackupForScope 创建一条备份记录并按需真实执行（OPS-008 数据面备份）。
//
//   - scope 非空（游戏域控制台）：必须解析到该游戏环境注册的启用数据源
//     （DBSourceModel.FindByScope），无则 404；解析成功后执行真实
//     dump→sha256→上传对象存储→更新记录（需 ObjectStore，缺则 500 不落记录，
//     避免悬挂 pending）。
//   - scope 为空（平台级调用）：退化为记录模式（与旧行为兼容，由平台备份页
//     管理 meta 库备份；此时若有 ObjectStore 则按 meta 库驱动/DSN 真实执行）。
func createBackupForScope(ctx context.Context, svcCtx *svc.ServiceContext, scope svc.GameScope, name, backupType string, timeoutSec int) (*model.Backup, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	if svcCtx.BackupModel == nil {
		return nil, errors.New("backup model unavailable")
	}
	backupType = strings.ToLower(strings.TrimSpace(backupType))
	if backupType == "" {
		backupType = "full"
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("%s-%s", backupType, time.Now().UTC().Format("20060102-150405"))
	}

	var src *model.DBSource
	if strings.TrimSpace(scope.GameID) != "" {
		if svcCtx.DBSourceModel == nil {
			return nil, errorx.NewInternalError("db source model unavailable")
		}
		found, err := svcCtx.DBSourceModel.FindByScope(ctx, scope.GameID, scope.Env)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewNotFound(fmt.Sprintf("no db source registered for %s/%s", scope.GameID, scope.Env))
			}
			return nil, err
		}
		src = found
	}

	exec := backupExecutorFor(svcCtx, src, scope)

	// 游戏域备份必须真实可执行：无执行器（ObjectStore 未配置 / 驱动不支持）
	// 时拒绝创建，不落悬挂 pending。
	if strings.TrimSpace(scope.GameID) != "" && exec == nil {
		return nil, errorx.NewInternalError("backup storage not configured")
	}

	backup := &model.Backup{
		BackupID: uuid.New().String(),
		Name:     name,
		Type:     backupType,
		Status:   "pending",
		GameID:   scope.GameID,
		Env:      scope.Env,
	}
	if src != nil {
		backup.SourceID = src.ID
		backup.SourceName = src.Name
	}
	if err := svcCtx.BackupModel.Create(ctx, backup); err != nil {
		return nil, err
	}

	if exec != nil {
		backupID, bname, btype := backup.BackupID, backup.Name, backup.Type
		go func() {
			// WithoutCancel 从入参派生：断开不取消备份执行，保留 ctx 值
			//（game DB 路由等）（CR-FIX-22）。
			bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), backupTimeout(timeoutSec))
			defer cancel()
			if err := exec.RunBackup(bg, backupID, bname, btype); err != nil {
				slog.Error("ops backup run failed", "backupID", backupID, "error", err)
			}
		}()
	}
	return backup, nil
}

// backupExecutorFor 构造真实备份执行器。src 非空 → 按注册数据源驱动/DSN 与
// scope 前缀；src 空（平台级）→ 回退 meta 库驱动/DSN。存储未就绪、Config 未
// 装配或驱动不支持 → nil。
func backupExecutorFor(svcCtx *svc.ServiceContext, src *model.DBSource, scope svc.GameScope) *backupexec.Executor {
	if svcCtx == nil || svcCtx.BackupModel == nil || svcCtx.ObjectStore == nil {
		return nil
	}
	driver, dsn, prefix := "", "", "backups"
	if src != nil {
		driver, dsn = src.Driver, src.DSN
		prefix = fmt.Sprintf("backups/%s/%s", scope.GameID, scope.Env)
	} else {
		driver, dsn = svcCtx.Config.Database.Driver, svcCtx.Config.Database.DataSource
	}
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mysql", "postgres", "pg", "sqlite":
		return backupexec.New(driver, dsn, svcCtx.BackupModel, svcCtx.ObjectStore, prefix)
	default:
		return nil
	}
}

// buildOpsBackupItems 映射模型行到 ops 清单 DTO（含 OPS-008 归属/校验列）。
func buildOpsBackupItems(backups []model.Backup) []Backup {
	items := make([]Backup, 0, len(backups))
	for _, b := range backups {
		items = append(items, Backup{
			Id:         fmt.Sprintf("%d", b.ID),
			Name:       b.Name,
			Type:       b.Type,
			Status:     b.Status,
			Size:       b.Size,
			CreatedAt:  utils.FormatTimestamp(b.CreatedAt),
			GameId:     b.GameID,
			Env:        b.Env,
			SourceName: b.SourceName,
			Checksum:   b.Checksum,
		})
	}
	return items
}
