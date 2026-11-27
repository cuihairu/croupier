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

// backupDedupeWindow is the dedupe window for same-name same-type backups when no
// client idempotency key is present (AUD-053): it guards against double clicks and
// rapid retries; normal ops cadence (scheduled same-name backups spaced >= the
// window) is unaffected.
const backupDedupeWindow = 10 * time.Minute

// Backup row status values used by the create/convergence paths. The executor
// (internal/platform/backupexec) writes the same two terminal values.
const (
	backupStatusPending = "pending"
	backupStatusFailed  = "failed"
)

// AUD-082: stable client-facing code and machine-readable reasons for the
// create-time rejection when no executor can ever run a backup.
const (
	backupExecutorUnavailableCode    = "backup_executor_unavailable"
	executorReasonBackupModelMissing = "backup_model_missing"
	executorReasonObjectStoreMissing = "object_store_missing"
	executorReasonDriverUnsupported  = "driver_unsupported"
)

// backupOrphanPendingGrace is the age after which a pending backup row can no
// longer be in flight (AUD-082): every runner bounds its run with a 30m context
// budget (generic create, scheduler, ops default), after which the row is advanced
// to succeeded/failed or the process died and the row is orphaned. The extra 15m
// slack covers dump/upload overshoot before cancellation propagates. Rows pending
// longer than this are converged to failed on read; a legitimately long ops run
// (caller-supplied timeout > grace) would be mis-statused transiently and
// self-heal when its run finishes.
const backupOrphanPendingGrace = 45 * time.Minute

// backupOrphanPendingMessage is the error_message recorded on rows converged by
// the AUD-082 read/retry convergence (and surfaced to clients via the backup DTO).
const backupOrphanPendingMessage = "备份长时间停留在 pending 且从未被执行器推进，已按悬挂记录收敛为失败；请重新创建备份"

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
	// AUD-083: the database backup row is the single record source of truth. The
	// legacy extension-mirror snapshot (config key "backups" of the
	// official.backup-advanced installation, written by the pre-AUD-083
	// dual-write) is migrated into the database here: a one-way, idempotent import
	// that runs before anything is read, so historical mirror entries survive the
	// cutover without any second read path. The mirror must never override the
	// database state, so the import only ever inserts rows whose id has no live
	// database row yet and drains those entries from the snapshot afterwards
	// (deleting an imported row can therefore never resurrect it).
	if err := s.importLegacyMirrorBackups(ctx); err != nil {
		return nil, err
	}
	// AUD-082 read-side convergence: pending rows older than the maximum run
	// budget can no longer be in flight (the runner died or the row was created
	// while no executor existed); they are flipped to failed here so they surface
	// as failed with a reason instead of pending forever. The convergence runs
	// after the legacy import, so imported rows get the same treatment in the
	// same read; the mirror itself is no longer converged (AUD-083: it is a
	// write-side legacy artifact, not a status source).
	if err := s.convergeOrphanPendingRows(ctx); err != nil {
		return nil, err
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
	// AUD-082: reject up-front when no executor can ever run this backup (object
	// storage not configured / database driver unsupported). Before this gate the
	// pending row was written first and only then was the executor inspected, so
	// the caller got a pending status that nothing would ever advance.
	if reason := s.executorUnavailableReason(); reason != "" {
		return nil, executorUnavailableError(reason)
	}
	backupType := strings.ToLower(strings.TrimSpace(req.Type))
	if backupType == "" {
		backupType = "full"
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("%s-%s", backupType, time.Now().UTC().Format("20060102-150405"))
	}

	// AUD-053 create idempotency: a backup with the same key that is active
	// (pending/succeeded) is reused as-is - retries/double clicks no longer stack
	// duplicate backups; failed does not count as active (a retry after failure
	// builds a new backup with the same key).
	// Without a client key, a name+type time window is the fallback (auto-generated
	// names carry a second-level timestamp, so same-second double clicks collide on
	// the same name and are caught by the window too).
	idemKey := strings.TrimSpace(req.IdempotencyKey)
	if idemKey != "" {
		if existing, err := s.svcCtx.BackupModel.FindActiveByIdempotencyKey(ctx, idemKey); err == nil && existing != nil {
			// AUD-082: a pending hit older than the run budget is an orphan (its
			// runner is long gone); converge it and fall through to a fresh,
			// actually executable backup instead of re-issuing a dead pending row.
			zombie, err := s.convergeZombiePending(ctx, existing)
			if err != nil {
				return nil, err
			}
			if !zombie {
				return &BackupCreateResponse{Backup: buildBackupDTO(existing)}, nil
			}
		}
		// A failed row with the same key yields: after a failure a retry creates a new
		// row and the unique index must not block it
		// (pre-check miss = no active row; a failed row may still be holding the key).
		// A just-converged zombie is failed too, so this release also lets its key go.
		if err := s.svcCtx.BackupModel.ReleaseIdempotencyKey(ctx, idemKey); err != nil {
			return nil, err
		}
	} else if existing, err := s.svcCtx.BackupModel.FindRecentDuplicateByNameType(ctx, name, backupType, backupDedupeWindow); err == nil && existing != nil {
		// AUD-082: same orphan rule as the idempotency-key branch above.
		zombie, err := s.convergeZombiePending(ctx, existing)
		if err != nil {
			return nil, err
		}
		if !zombie {
			return &BackupCreateResponse{Backup: buildBackupDTO(existing)}, nil
		}
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
		// Concurrent double-write race: after both pre-checks miss, a same-key insert
		// lands on the unique index as the safety net - the later arrival falls back to
		// the existing row and the caller still sees one successful idempotent create.
		if idemKey != "" && isIdempotencyConflict(err) {
			if existing, ferr := s.svcCtx.BackupModel.FindActiveByIdempotencyKey(ctx, idemKey); ferr == nil && existing != nil {
				return &BackupCreateResponse{Backup: buildBackupDTO(existing)}, nil
			}
		}
		return nil, err
	}
	// AUD-083: no mirror write here anymore. The pending row lives in the
	// database only and the async executor below advances that same row, so the
	// status the caller later reads back is the status the executor wrote. (The
	// legacy extension mirror is migrated on read by importLegacyMirrorBackups.)
	_ = s.recordBackupEvent(ctx, "backups_create", "backup created",
		fmt.Sprintf(`{"backup_id":"%s","type":"%s"}`, backup.BackupID, backup.Type),
	)

	// Real execution (async): export -> verify -> upload to object storage -> update
	// the record. On failure the status is set to failed with the error message kept;
	// the caller immediately gets the pending status.
	if exec := s.executor(); exec != nil {
		backupID, name, backupType := backup.BackupID, backup.Name, backup.Type
		go func() {
			// WithoutCancel derives from the input ctx: the async backup is decoupled
			// from the request lifecycle (a client disconnect does not abort it) while
			// ctx values are preserved (game DB routing etc.) (CR-FIX-22).
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

// isIdempotencyConflict recognizes idempotency-key unique-index conflicts (AUD-053):
// all three dialects carry the column name in the error text (sqlite "UNIQUE
// constraint failed: backups.idempotency_key",
// postgres "duplicate key value violates unique constraint ...idempotency_key",
// mysql "Duplicate entry ... for key 'idempotency_key'").
func isIdempotencyConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "idempotency_key")
}

// executorUnavailableReason reports why the generic backup entry cannot execute
// a backup at all (AUD-082). "" means executor() would return a real executor;
// otherwise the returned value is the stable machine-readable reason carried in
// the 400 rejection details ("backup_model_missing" is defensive and unreachable
// from Create, which checks the model first).
func (s *Service) executorUnavailableReason() string {
	if s == nil || s.svcCtx == nil || s.svcCtx.BackupModel == nil {
		return executorReasonBackupModelMissing
	}
	if s.svcCtx.ObjectStore == nil {
		return executorReasonObjectStoreMissing
	}
	if !backupexec.SupportedDriver(s.svcCtx.Config.Database.Driver) {
		return executorReasonDriverUnsupported
	}
	return ""
}

// executorUnavailableError builds the AUD-082 create-time rejection: 400 with the
// stable snake_case code and a machine-readable reason in details, per the repo
// API response contract.
func executorUnavailableError(reason string) error {
	message := "备份执行器不可用，无法创建备份"
	switch reason {
	case executorReasonObjectStoreMissing:
		message = "对象存储未配置，备份无法执行；请先完成 storage 配置后再创建备份"
	case executorReasonDriverUnsupported:
		message = "数据库驱动不支持备份执行（支持 mysql/postgres/sqlite），已拒绝创建"
	}
	return errorx.NewBadRequestWithCode(
		backupExecutorUnavailableCode,
		message,
		map[string]any{"reason": reason},
	)
}

// executor builds the real backup executor. Since AUD-082 the create path rejects
// up-front when this would return nil, so a pending record is only ever written
// when a real executor exists (record-only create mode no longer exists).
func (s *Service) executor() *backupexec.Executor {
	if s.executorUnavailableReason() != "" {
		return nil
	}
	return backupexec.New(
		s.svcCtx.Config.Database.Driver,
		s.svcCtx.Config.Database.DataSource,
		s.svcCtx.BackupModel,
		s.svcCtx.ObjectStore,
		"backups",
	)
}

// convergeZombiePending implements the AUD-082 retry-side convergence: a dedupe/
// idempotency hit whose status is still pending but whose age exceeds the maximum
// run budget is an orphan (no runner is going to advance it). It is flipped to
// failed with the reason text and the caller falls through to create a fresh,
// actually executable backup. Returns false when the row is not a zombie (reuse
// it as before) — a convergence write error is returned, not swallowed.
func (s *Service) convergeZombiePending(ctx context.Context, existing *model.Backup) (bool, error) {
	if !isOrphanPending(existing, time.Now().UTC().Add(-backupOrphanPendingGrace)) {
		return false, nil
	}
	if err := s.svcCtx.BackupModel.UpdateByBackupID(ctx, existing.BackupID, map[string]interface{}{
		"status":        backupStatusFailed,
		"error_message": backupOrphanPendingMessage,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// convergeOrphanPendingRows is the AUD-082 read-side convergence for the legacy
// rows written by the old write-pending-then-inspect bug (plus crash/restart
// orphans): every pending row older than backupOrphanPendingGrace is flipped to
// failed with the reason text. Rows that may still be in flight (created within
// the grace window) are never touched.
//
// AUD-083: this now covers every pending database row regardless of origin —
// legacy bug rows, crash orphans and entries imported from the extension mirror
// by importLegacyMirrorBackups (which runs before this convergence on each list).
// The former mirror-side convergence half is retired: the mirror is no longer a
// status source, and the database convergence subsumes its migrated rows.
func (s *Service) convergeOrphanPendingRows(ctx context.Context) error {
	if s == nil || s.svcCtx == nil || s.svcCtx.BackupModel == nil || s.svcCtx.DB == nil {
		return nil
	}
	// The cutoff is bound in local wall-clock shape (no .UTC() conversion): the
	// sqlite driver stores/compares timestamps as offset-bearing text, so a UTC
	// ("Z"-suffixed) literal never compares correctly against rows stored with a
	// local offset. Binding both sides in the same shape keeps the comparison
	// consistent on every dialect (same pattern as the AUD-053 dedupe window).
	if err := s.svcCtx.DB.WithContext(ctx).Model(&model.Backup{}).
		Where("status = ? AND created_at <= ?", backupStatusPending, time.Now().Add(-backupOrphanPendingGrace)).
		Updates(map[string]interface{}{
			"status":        backupStatusFailed,
			"error_message": backupOrphanPendingMessage,
		}).Error; err != nil {
		return err
	}
	return nil
}

// importLegacyMirrorBackups performs the AUD-083 one-way migration of the legacy
// extension-mirror snapshot (config key "backups" of the official.backup-advanced
// installation) into the database. Before AUD-083, List preferred that snapshot
// while the executor advanced database rows, so the two stores disagreed forever.
// The database row is now the single source of truth, and this migration is the
// documented compatibility story for existing mirror-only history:
//
//   - a mirror entry whose id has no database row is imported as a new row,
//     preserving name/type/size/status/errorMessage and the RFC3339 createdAt;
//   - a mirror entry whose id already has a live database row is redundant (the
//     row is authoritative — the executor never updated the mirror), so the stale
//     mirror copy is drained;
//   - every migrated entry is removed from the snapshot in the same pass (the
//     config is rewritten without it), which makes the import idempotent: after a
//     successful pass the snapshot no longer feeds the import loop, and deleting
//     an imported row can never resurrect it from a leftover mirror copy. The
//     other config keys and the secret refs are preserved (see
//     saveBackupsToExtensionInstallation);
//   - entries without a usable id cannot be addressed by any API (delete/download
//     key on the id) and stay in the snapshot untouched — a documented boundary;
//   - imported pending entries keep the pending status and follow the AUD-082
//     age-based convergence: entries older than the grace window flip to failed
//     on this very list (the convergence runs right after the import), fresh ones
//     stay pending until the window passes and converge on a later read. An
//     unparseable/absent createdAt imports with the import time and behaves the
//     same way — an unprovable orphan is not failed by a heuristic.
//
// Import and drain errors are returned, not swallowed, so an underlying database
// or extension-config failure surfaces to the caller instead of silently leaving
// the migration half-done. A backup_id unique-index conflict from a concurrent
// import/create racing the same id is treated as already migrated.
func (s *Service) importLegacyMirrorBackups(ctx context.Context) error {
	items, ok, err := s.loadBackupsFromExtensionInstallation(ctx)
	if err != nil || !ok || len(items) == 0 {
		return err
	}
	remaining := make([]Backup, 0, len(items))
	changed := false
	for i := range items {
		id := strings.TrimSpace(items[i].Id)
		if id == "" {
			remaining = append(remaining, items[i])
			continue
		}
		if _, ferr := s.svcCtx.BackupModel.FindByBackupID(ctx, id); ferr == nil {
			// A live database row owns this id: drain the stale mirror copy.
			changed = true
			continue
		} else if !errors.Is(ferr, gorm.ErrRecordNotFound) {
			return ferr
		}
		if err := s.svcCtx.BackupModel.Create(ctx, mirrorEntryToRow(&items[i])); err != nil {
			if isBackupIDConflict(err) {
				changed = true
				continue
			}
			return err
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return s.saveBackupsToExtensionInstallation(ctx, remaining)
}

// mirrorEntryToRow maps a legacy extension-mirror entry onto a database backup
// row for the one-way import.
func mirrorEntryToRow(entry *Backup) *model.Backup {
	row := &model.Backup{
		BackupID:     strings.TrimSpace(entry.Id),
		Name:         strings.TrimSpace(entry.Name),
		Size:         entry.Size,
		Type:         strings.ToLower(strings.TrimSpace(entry.Type)),
		Status:       strings.TrimSpace(entry.Status),
		ErrorMessage: entry.ErrorMessage,
	}
	if row.Name == "" {
		row.Name = row.BackupID
	}
	if created, err := time.Parse(time.RFC3339, strings.TrimSpace(entry.CreatedAt)); err == nil && !created.IsZero() {
		// Bind in local wall-clock shape: the sqlite driver stores/compares
		// timestamps as offset-bearing text, and an imported UTC ("Z") literal
		// never compares correctly against the AUD-082 convergence cutoff built
		// from time.Now() (same reason as the cutoff binding in
		// convergeOrphanPendingRows). Without .Local() a just-imported fresh
		// entry would be mis-flipped to failed on the very first list.
		row.CreatedAt = created.Local()
	}
	return row
}

// isBackupIDConflict recognizes backup_id unique-index conflicts raised by the
// legacy-mirror import when a concurrent import/create inserted the same id
// first; the loser treats the row as already migrated instead of failing the
// list (same dialect shape as isIdempotencyConflict: all three dialects carry
// the column/constraint name in the error text).
func isBackupIDConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "backup_id")
}

// isOrphanPending reports whether the row is a pending record that can no longer
// be in flight: status pending and created before the cutoff.
func isOrphanPending(backup *model.Backup, cutoff time.Time) bool {
	return backup != nil &&
		strings.EqualFold(strings.TrimSpace(backup.Status), backupStatusPending) &&
		!backup.CreatedAt.IsZero() &&
		backup.CreatedAt.Before(cutoff)
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
	// AUD-083: no mirror cleanup here anymore. The one-way import drains migrated
	// entries from the extension snapshot and new records never enter it, so the
	// snapshot cannot resurrect a deleted row (the delete identity comes from the
	// database row, which the 404 above already proved existed).
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

	// AUD-COV-001 P1: every other shape is read through objstore by object key. The
	// original implementation trusted the record's location, accepting file:// and any
	// absolute/relative path to read the local filesystem directly, which could be
	// coaxed into reading arbitrary server files; the objstore drivers carry their own
	// within-base-directory validation (the file driver's validateAndCleanPath), the
	// same semantics as the backup executor's write/restore path.
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

// objectSize fetches the object size via a List prefix query with exact matching
// (the Store interface has no Stat). The prefix is the key's parent directory, not
// the full key: the file driver's List walks with the directory as root, so a full
// key hits the "skip the root itself" branch and yields no entry; object-storage
// drivers list same-directory objects by prefix. A root-level key (no "/") lists
// everything with an empty prefix, paging by marker as the fallback.
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
		Id:           backup.BackupID,
		Name:         backup.Name,
		Size:         backup.Size,
		Type:         backup.Type,
		Status:       backup.Status,
		CreatedAt:    utils.FormatTimestamp(backup.CreatedAt),
		ErrorMessage: backup.ErrorMessage,
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
	// raw comes from json.Unmarshal(map[string]any), so value types can only be the
	// JSON basic types (nil/bool/float64/string/[]any/map[string]any); re-Marshal
	// always succeeds, the error branch is unreachable and was removed.
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
	// AUD-083: an empty rewrite means the legacy snapshot fully drained into the
	// database — drop the key instead of leaving an empty array behind, so the
	// import fast-path skips the installation entirely on later reads.
	if len(items) == 0 {
		delete(config, backupRecordsKey)
	} else {
		config[backupRecordsKey] = items
	}
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

// AUD-083 note: the former mirror-maintenance helpers (upsert/remove on create
// and delete) and the mirror-only filter/pagination helpers were removed with
// the dual-write; list paging/filtering now goes through BackupModel.List
// (model.NewPagination normalizes page/pageSize for the response).
