// Coverage targets: the executor supported branch and default branch, Create's async
// RunBackup success/failure events, Delete's DeleteByBackupID error, resolveBackupPath's
// Abs failure, findActiveBackupInstallation's List error, saveBackups's UpdateConfig
// error, and upsert/remove's extension record loading errors.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync/atomic"
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

// ---- test doubles ----

type execObjStoreStub struct {
	putErr error
}

func (s *execObjStoreStub) Put(ctx context.Context, key string, r objstore.ReadSeeker, size int64, contentType string) error {
	return s.putErr
}
func (s *execObjStoreStub) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
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

// backupExecEnvSeq hands out a uniquely named in-memory database per call. Naming by
// t.Name() is not enough: -count=3 runs the same case name three times, and if the
// previous run's connection pool has not fully closed, a database with the same name
// is still alive, so inserting the same BackupID this run hits the UNIQUE constraint
// (backups.backup_id 2067).
var backupExecEnvSeq atomic.Int64

// newExecTestEnv builds an env with an isolated in-memory database (not sharing
// backupFlowDB), with driver/dataSource controllable to drive executor branches.
func newExecTestEnv(t *testing.T, driver, dataSource string, store objstore.Store) *backupFlowEnv {
	t.Helper()
	dbName := fmt.Sprintf("execenv_%d", backupExecEnvSeq.Add(1))
	db, err := gorm.Open(gsqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	seedBackupSuperAdmin(t, db)

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)

	svcCtx := &svc.ServiceContext{
		DB:          db,
		AdminModel:  model.NewAdminModel(db),
		RoleModel:   model.NewRoleModel(db),
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

// writeRealSQLiteDB creates a real SQLite database: the backup executor uses VACUUM
// INTO to produce a consistent snapshot, so the source must be a valid database file
// (passing off plain text fails outright, AUD-009).
func writeRealSQLiteDB(t *testing.T, path string) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE demo (id INTEGER PRIMARY KEY, name TEXT)").Error)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Exec("INSERT INTO demo (name) VALUES (?)", fmt.Sprintf("row-%d", i)).Error)
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

func TestService_Create_AsyncRunSucceeds(t *testing.T) {
	src := filepath.Join(t.TempDir(), "game.db")
	writeRealSQLiteDB(t, src)

	env := newExecTestEnv(t, "sqlite", src, &execObjStoreStub{})
	resp, err := env.service.Create(env.authCtx(t), &BackupCreateRequest{Name: "async-ok"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	waitBackupStatus(t, env, resp.Id, "succeeded")
}

func TestService_Create_AsyncRunFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	env := newExecTestEnv(t, "sqlite", missing, &execObjStoreStub{})
	resp, err := env.service.Create(env.authCtx(t), &BackupCreateRequest{Name: "async-fail"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	waitBackupStatus(t, env, resp.Id, "failed")
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
	err := env.service.Delete(env.authCtx(t), &BackupDeleteRequest{ID: "bkp-del-fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced delete failure")
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

	// UpdateConfig runs Get(query) first internally, and Save goes through the update
	// callback.
	injectBackupCallback(t, env.db, "update")
	err := env.service.saveBackupsToExtensionInstallation(context.Background(), []Backup{{Id: "bkp-save"}})
	require.Error(t, err)
}
