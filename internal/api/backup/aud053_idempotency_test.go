package backup

// AUD-053 备份创建幂等键测试：
// - 同键顺序重试 → 复用已存在行（pending/succeeded 算活跃）
// - 并发同键多写 → 唯一索引兜底，全员拿同一行、库里只有一行
// - failed 行不算活跃 → 同键重试建新备份
// - 无键时 name+type 时间窗兜底（双击）；不同类型/超窗不误伤
// 断言不依赖列表顺序；并发用例走独立库（SetMaxOpenConns(1) 串行化连接，
// 竞争路径仍被覆盖：预检落空 → 插入冲突 → 回落）。
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

// aud053Env 独立内存库（单连接）：并发用例不被 sqlite 共享缓存锁拖成假红；
// 超管种子复用 flows 包助手，让服务层 RequireAnyPermission 直通。
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

// 同键顺序重试复用：第二次 Create 返回同一备份，库里只有一行。
func TestAud053_IdempotencyKeySequentialRetryReuses(t *testing.T) {
	s, db, ctx := aud053Env(t)

	first, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-seq", Type: "full", IdempotencyKey: "aud053-key-seq"})
	require.NoError(t, err)
	second, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-seq-retry", Type: "full", IdempotencyKey: "aud053-key-seq"})
	require.NoError(t, err)

	assert.Equal(t, first.Backup.Id, second.Backup.Id, "同键重试必须复用已存在行")
	assert.EqualValues(t, 1, aud053CountByKey(t, db, "aud053-key-seq"))
}

// succeeded 也算活跃：执行完成的备份同键重试同样复用（不叠加重备份）。
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

// failed 不算活跃：失败后同键重试创建新备份（重试语义）。
func TestAud053_FailedBackupAllowsRetryWithSameKey(t *testing.T) {
	s, db, ctx := aud053Env(t)

	first, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-fail", Type: "full", IdempotencyKey: "aud053-key-fail"})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.Backup{}).Where("backup_id = ?", first.Backup.Id).
		Update("status", "failed").Error)

	second, err := s.Create(ctx, &BackupCreateRequest{Name: "aud053-fail-retry", Type: "full", IdempotencyKey: "aud053-key-fail"})
	require.NoError(t, err)
	assert.NotEqual(t, first.Backup.Id, second.Backup.Id, "失败后同键重试必须建新备份")
	// 旧 failed 行已让位键（NULL），键上只剩新行——后续同键请求继续正确幂等。
	assert.EqualValues(t, 1, aud053CountByKey(t, db, "aud053-key-fail"))
}

// 并发同键多写：预检竞争双双落空 → 唯一索引兜底 → 全员同一行、库里一行。
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

// 无键时 name+type 时间窗兜底：同名同类型窗内复用；不同类型不误伤。
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

// 超窗不复用：同名同类型但行已老于窗口 → 新建（正常运维节奏不受影响）。
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
