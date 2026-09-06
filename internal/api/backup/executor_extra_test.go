// 覆盖目标：executor 支持分支与默认分支、Create 的异步 RunBackup 成功/
// 失败事件、Delete 的 DeleteByBackupID 错误、resolveBackupPath 的 Abs 失败、
// findActiveBackupInstallation 的 List 错误、saveBackups 的 UpdateConfig 错误、
// upsert/remove 的扩展记录加载错误。
package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- 测试替身 ----

type execObjStoreStub struct {
	putErr error
}

func (s *execObjStoreStub) Put(ctx context.Context, key string, r objstore.ReadSeeker, size int64, contentType string) error {
	return s.putErr
}
func (s *execObjStoreStub) SignedURL(ctx context.Context, key string, method string, expiry time.Duration) (string, error) {
	return "", nil
}
func (s *execObjStoreStub) Delete(ctx context.Context, key string) error { return nil }
func (s *execObjStoreStub) List(ctx context.Context, prefix, marker, delimiter string, limit int) (objstore.ListResult, error) {
	return objstore.ListResult{}, nil
}
func (s *execObjStoreStub) CreatePrefix(ctx context.Context, prefix string) error { return nil }
func (s *execObjStoreStub) RenamePrefix(ctx context.Context, oldPrefix, newPrefix string) error {
	return nil
}

// newExecTestEnv 建独立内存库 env（不共享 backupFlowDB），driver/dataSource
// 可控以驱动 executor 分支。
func newExecTestEnv(t *testing.T, driver, dataSource string, store objstore.Store) *backupFlowEnv {
	t.Helper()
	dbName := "execenv_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(gsqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)

	svcCtx := &svc.ServiceContext{
		DB:          db,
		BackupModel: model.NewBackupModel(db),
		Extensions:  &svc.ExtensionServices{Installation: installationSvc},
		Config: config.Config{
			Database: config.DatabaseConfig{Driver: driver, DataSource: dataSource},
		},
	}
	if store != nil {
		svcCtx.ObjectStore = store
	}
	service := NewService(svcCtx)
	return &backupFlowEnv{db: db, svcCtx: svcCtx, service: service}
}

func waitBackupStatus(t *testing.T, env *backupFlowEnv, backupID, want string) {
	t.Helper()
	require.Eventually(t, func() bool {
		rec, err := env.svcCtx.BackupModel.FindByBackupID(context.Background(), backupID)
		return err == nil && rec.Status == want
	}, 15*time.Second, 100*time.Millisecond, "backup %s should become %s", backupID, want)
}

func TestExecutor_SupportedDriver(t *testing.T) {
	env := newExecTestEnv(t, "sqlite", "/dev/null", &execObjStoreStub{})
	exec := env.service.executor()
	require.NotNil(t, exec, "sqlite + ObjectStore 应构造真实执行器")
}

func TestExecutor_UnsupportedDriver(t *testing.T) {
	env := newExecTestEnv(t, "oracle", "dsn", &execObjStoreStub{})
	assert.Nil(t, env.service.executor(), "不支持的 driver 应回退纯记录模式")
}

func TestService_Create_AsyncRunSucceeds(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "game.db")
	require.NoError(t, os.WriteFile(src, []byte("sqlite-payload"), 0o600))

	env := newExecTestEnv(t, "sqlite", src, &execObjStoreStub{})
	resp, err := env.service.Create(context.Background(), &BackupCreateRequest{Name: "async-ok"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	waitBackupStatus(t, env, resp.Backup.Id, "succeeded")
}

func TestService_Create_AsyncRunFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	env := newExecTestEnv(t, "sqlite", missing, &execObjStoreStub{})
	resp, err := env.service.Create(context.Background(), &BackupCreateRequest{Name: "async-fail"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	waitBackupStatus(t, env, resp.Backup.Id, "failed")
}

func injectBackupCallback(t *testing.T, db *gorm.DB, op string) {
	t.Helper()
	name := "test:fail_" + op
	fn := func(tx *gorm.DB) { _ = tx.AddError(errors.New("forced " + op + " failure")) }
	switch op {
	case "delete":
		require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(name, fn))
	case "update":
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, fn))
	default:
		t.Fatalf("unsupported op %q", op)
	}
	t.Cleanup(func() {
		_ = db.Callback().Delete().Remove(name)
		_ = db.Callback().Update().Remove(name)
	})
}

func TestService_Delete_DeleteByBackupIDError(t *testing.T) {
	env := newExecTestEnv(t, "", "", nil)
	createBackupRow(t, env, &model.Backup{BackupID: "bkp-del-fail", Name: "n", Type: "full"})

	injectBackupCallback(t, env.db, "delete")
	err := env.service.Delete(context.Background(), &BackupDeleteRequest{ID: "bkp-del-fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced delete failure")
}

func TestResolveBackupPath_RelativeCwdDeleted(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.RemoveAll(wd))

	env := newExecTestEnv(t, "", "", nil)
	_, err := env.service.resolveBackupPath("relative.tar")
	require.Error(t, err, "cwd 被删后相对路径 Abs 应失败")
}

func TestFindActiveBackupInstallation_ListError(t *testing.T) {
	env := newExecTestEnv(t, "", "", nil)
	require.NoError(t, env.db.Migrator().DropTable("extension_installations"))

	_, _, err := env.service.findActiveBackupInstallation(context.Background())
	require.Error(t, err)
}

func corruptActiveInstallationConfig(t *testing.T, env *backupFlowEnv) {
	t.Helper()
	item := installBackupAdvancedExtension(t, env, map[string]any{})
	require.NoError(t, env.db.Exec("UPDATE extension_installations SET config_json = ? WHERE id = ?",
		`{invalid-json`, item.ID).Error)
}

func TestUpsertBackupToExtension_LoadError(t *testing.T) {
	env := newExecTestEnv(t, "", "", nil)
	corruptActiveInstallationConfig(t, env)

	err := env.service.upsertBackupToExtension(context.Background(), Backup{Id: "bkp-x", Name: "n"})
	require.Error(t, err)
}

func TestRemoveBackupFromExtension_LoadError(t *testing.T) {
	env := newExecTestEnv(t, "", "", nil)
	corruptActiveInstallationConfig(t, env)

	err := env.service.removeBackupFromExtension(context.Background(), "bkp-x")
	require.Error(t, err)
}

func TestSaveBackupsToExtensionInstallation_UpdateConfigError(t *testing.T) {
	env := newExecTestEnv(t, "", "", nil)
	installBackupAdvancedExtension(t, env, map[string]any{})

	// UpdateConfig 内部 Get(query) 先行，Save 走 update 回调。
	injectBackupCallback(t, env.db, "update")
	err := env.service.saveBackupsToExtensionInstallation(context.Background(), []Backup{{Id: "bkp-save"}})
	require.Error(t, err)
}
