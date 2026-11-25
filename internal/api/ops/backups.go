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

// Verify recomputes the sha256 of the backup object and compares it with the
// recorded checksum (OPS-008 verification read path). A true return means the
// checksums match; read/hash failures return an error.
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

// createBackupForScope creates one backup record and actually runs it when
// possible (OPS-008 data-plane backups).
//
//   - scope non-empty (game-domain console): it must resolve to an enabled data
//     source registered for that game env (DBSourceModel.FindByScope), otherwise
//     404; after a successful resolution a real dump→sha256→object-store
//     upload→record update runs (requires ObjectStore; without one it 500s and
//     writes no record, avoiding a dangling pending).
//   - scope empty (platform-level call): degrades to record-only mode
//     (compatible with the old behavior; the platform backup page manages meta
//     database backups, and with an ObjectStore present it really executes using
//     the meta database driver/DSN).
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

	// Game-domain backups must be really executable: reject creation when no
	// executor is available (ObjectStore not configured / driver unsupported)
	// so no dangling pending lands.
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
			// WithoutCancel derives from the input: a disconnect does not cancel
			// the backup run, and ctx values (game DB routing etc.) are kept
			// (CR-FIX-22).
			bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), backupTimeout(timeoutSec))
			defer cancel()
			if err := exec.RunBackup(bg, backupID, bname, btype); err != nil {
				slog.Error("ops backup run failed", "backupID", backupID, "error", err)
			}
		}()
	}
	return backup, nil
}

// backupExecutorFor builds a real backup executor. src non-nil -> use the
// registered data source's driver/DSN with a scope prefix; src nil
// (platform-level) -> fall back to the meta database driver/DSN. Storage not
// ready, Config not assembled, or an unsupported driver -> nil.
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

// buildOpsBackupItems maps model rows to the ops list DTO (including the
// OPS-008 ownership/verification columns).
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
