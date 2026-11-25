package backup

// AUD-053 backup creation idempotency key tests:
// - sequential retry with the same key -> reuses the existing row (pending/succeeded
//   count as active)
// - concurrent writes with the same key -> the unique index is the fallback; everyone
//   gets the same row and only one row lands in the database
// - a failed row does not count as active -> a same-key retry creates a new backup
// - without a key, a name+type time window is the fallback (double click); different
//   types / expired windows are not falsely caught
// Assertions do not depend on list order; concurrency cases use an isolated database
// (SetMaxOpenConns(1) serializes connections while the competing path is still
// covered: precheck misses -> insert conflict -> fall back to reuse).
import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// aud053Env is an isolated in-memory database (single connection): concurrency cases
// are not dragged into a false red by the sqlite shared-cache lock; the super-admin
// seed reuses the flows package helper so the service-layer RequireAnyPermission
// passes through.
func aud053Env(t *testing.T) (*Service, *gorm.DB, context.Context) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, model.AutoMigrate(db))
	seedBackupSuperAdmin(t, db)
	svcCtx := &svc.ServiceContext{
		DB:          db,
		AdminModel:  model.NewAdminModel(db),
		RoleModel:   model.NewRoleModel(db),
		BackupModel: model.NewBackupModel(db),
	}
	ctx := context.WithValue(context.Background(), "username", backupSuperAdminUsername)
	return NewService(svcCtx), db, ctx
}

func aud053CountByKey(t *testing.T, db *gorm.DB, key string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.Backup{}).Where("idempotency_key = ?", key).Count(&n).Error)
	return n
}

func aud053CountByName(t *testing.T, db *gorm.DB, name string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.Backup{}).Where("name = ?", name).Count(&n).Error)
	return n
}

// Sequential retry with the same key reuses: the second Create returns the same backup
// and only one row lands in the database.
func TestAud053_IdempotencyKeySequentialRetryReuses(t *testing.T) {
	s, db, ctx := aud053Env(t)

	first, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-seq", Type: "full", IdempotencyKey: "aud053-key-seq"})
	require.NoError(t, err)
	second, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-seq-retry", Type: "full", IdempotencyKey: "aud053-key-seq"})
	require.NoError(t, err)

	assert.Equal(t, first.Backup.Id, second.Backup.Id, "同键重试必须复用已存在行")
	assert.EqualValues(t, 1, aud053CountByKey(t, db, "aud053-key-seq"))
}

// succeeded also counts as active: a completed backup reuses on a same-key retry
// (no stacking of duplicate backups).
func TestAud053_SucceededBackupStillDeduplicates(t *testing.T) {
	s, db, ctx := aud053Env(t)

	first, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-done", Type: "full", IdempotencyKey: "aud053-key-done"})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.Backup{}).Where("backup_id = ?", first.Backup.Id).
		Update("status", "succeeded").Error)

	second, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-done-again", Type: "full", IdempotencyKey: "aud053-key-done"})
	require.NoError(t, err)
	assert.Equal(t, first.Backup.Id, second.Backup.Id)
	assert.EqualValues(t, 1, aud053CountByKey(t, db, "aud053-key-done"))
}

// failed does not count as active: after a failure a same-key retry creates a new
// backup (retry semantics).
func TestAud053_FailedBackupAllowsRetryWithSameKey(t *testing.T) {
	s, db, ctx := aud053Env(t)

	first, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-fail", Type: "full", IdempotencyKey: "aud053-key-fail"})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.Backup{}).Where("backup_id = ?", first.Backup.Id).
		Update("status", "failed").Error)

	second, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-fail-retry", Type: "full", IdempotencyKey: "aud053-key-fail"})
	require.NoError(t, err)
	assert.NotEqual(t, first.Backup.Id, second.Backup.Id, "失败后同键重试必须建新备份")
	// The old failed row has released the key (NULL), leaving only the new row on the
	// key - later same-key requests stay correctly idempotent.
	assert.EqualValues(t, 1, aud053CountByKey(t, db, "aud053-key-fail"))
}

// Concurrent same-key writes: the racing prechecks both miss -> the unique index is
// the fallback -> everyone gets one row and the database holds a single row.
func TestAud053_ConcurrentSameKeyCreatesSingleRow(t *testing.T) {
	s, db, ctx := aud053Env(t)

	const n = 8
	responses := make([]*BackupCreateResponse, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	var start sync.WaitGroup
	start.Add(1)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start.Wait()
			responses[i], errs[i] = s.Create(ctx, &BackupCreateRequest{
				Name:           fmt.Sprintf("aud053-conc-%d", i),
				Type:           "full",
				IdempotencyKey: "aud053-key-conc",
			})
		}(i)
	}
	start.Done()
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i], "并发同键创建必须全部成功（冲突方回落复用）")
	}
	for i := 1; i < n; i++ {
		assert.Equal(t, responses[0].Backup.Id, responses[i].Backup.Id, "全员必须拿到同一行")
	}
	assert.EqualValues(t, 1, aud053CountByKey(t, db, "aud053-key-conc"), "库里必须只有一行")
}

// Without a key, the name+type time window is the fallback: same name and type reuse
// within the window; different types are not falsely caught.
func TestAud053_NameTypeWindowDedupe(t *testing.T) {
	s, db, ctx := aud053Env(t)

	first, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-nightly", Type: "full"})
	require.NoError(t, err)
	second, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-nightly", Type: "full"})
	require.NoError(t, err)
	assert.Equal(t, first.Backup.Id, second.Backup.Id, "窗内同名同类型双击必须复用")
	assert.EqualValues(t, 1, aud053CountByName(t, db, "aud053-nightly"))

	incremental, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-nightly", Type: "incremental"})
	require.NoError(t, err)
	assert.NotEqual(t, first.Backup.Id, incremental.Backup.Id, "不同类型是不同备份，不受窗口误伤")
}

// Expired window does not reuse: same name and type but the row is older than the
// window -> create fresh (normal operations cadence is unaffected).
func TestAud053_ExpiredWindowAllowsNewBackup(t *testing.T) {
	s, db, ctx := aud053Env(t)

	first, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-old", Type: "full"})
	require.NoError(t, err)
	stale := time.Now().Add(-(backupDedupeWindow + time.Minute))
	require.NoError(t, db.Model(&model.Backup{}).Where("backup_id = ?", first.Backup.Id).
		Update("created_at", stale).Error)

	second, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-old", Type: "full"})
	require.NoError(t, err)
	assert.NotEqual(t, first.Backup.Id, second.Backup.Id, "超窗后同名必须新建")
	assert.EqualValues(t, 2, aud053CountByName(t, db, "aud053-old"))
}
