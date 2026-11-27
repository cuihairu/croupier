package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type backupFlowEnv struct {
	db      *gorm.DB
	svcCtx  *svc.ServiceContext
	service *Service
	handler *Handler
	router  *gin.Engine
}

// backupSuperAdminUsername is the super-admin account used by the backup tests (the
// HasAdminRole pass-through branch); it is created idempotently once in the shared
// database so the service-layer RequireAnyPermission authorization check passes.
const backupSuperAdminUsername = "backup_superadmin"

var backupFlowDB *gorm.DB

func backupFlowSharedDB(t *testing.T) *gorm.DB {
	t.Helper()
	if backupFlowDB == nil {
		var err error
		backupFlowDB, err = gorm.Open(gsqlite.Open("file:backupflow?mode=memory&cache=shared"), &gorm.Config{})
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		if err := model.AutoMigrate(backupFlowDB); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	backupFlowDB.Exec("DELETE FROM extension_events")
	backupFlowDB.Exec("DELETE FROM extension_runtime_bindings")
	backupFlowDB.Exec("DELETE FROM extension_installations")
	backupFlowDB.Exec("DELETE FROM backups")
	return backupFlowDB
}

// seedBackupSuperAdmin idempotently creates the super-admin account named
// backupSuperAdminUsername and returns the username. The super_admin role is
// authorized through HasAdminRole pass-through with no specific permission rows; this
// keeps existing cases passing after the newly added service-layer authorization gate.
func seedBackupSuperAdmin(t *testing.T, db *gorm.DB) string {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin, err := adminModel.FindByUsername(ctx, backupSuperAdminUsername)
	if err != nil {
		admin = &model.Admin{Username: backupSuperAdminUsername, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}

	hasSuperAdmin := false
	roles, err := adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "super_admin" {
			hasSuperAdmin = true
			break
		}
	}
	if !hasSuperAdmin {
		role := &model.Role{Name: "super_admin"}
		require.NoError(t, roleModel.Create(ctx, role))
		require.NoError(t, adminModel.AssignRole(ctx, admin.ID, role.ID))
	}
	return backupSuperAdminUsername
}

// authCtx returns a request context carrying the super-admin username, satisfying the
// service-layer RequireAnyPermission.
func (e *backupFlowEnv) authCtx(t *testing.T) context.Context {
	t.Helper()
	return context.WithValue(context.Background(), "username", backupSuperAdminUsername)
}

func setupBackupFlowEnv(t *testing.T) *backupFlowEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := backupFlowSharedDB(t)
	seedBackupSuperAdmin(t, db)

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)

	// AUD-082: since the create path rejects when no executor is available, the
	// shared flow env is wired executor-capable (sqlite driver + real source file
	// + stub object store) so Create-shaped cases exercise the real acceptance
	// path; cases that need a specific store overwrite ObjectStore explicitly.
	src := filepath.Join(t.TempDir(), "flow-src.db")
	writeRealSQLiteDB(t, src)
	svcCtx := &svc.ServiceContext{
		DB:          db,
		AdminModel:  model.NewAdminModel(db),
		RoleModel:   model.NewRoleModel(db),
		BackupModel: model.NewBackupModel(db),
		Extensions:  &svc.ExtensionServices{Installation: installationSvc},
		Config: config.Config{
			Database: config.DatabaseConfig{Driver: "sqlite", DataSource: src},
		},
		ObjectStore: &execObjStoreStub{},
	}
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/api/v1/backups", handler.List)
	router.POST("/api/v1/backups", handler.Create)
	router.DELETE("/api/v1/backups/:id", handler.Delete)
	router.GET("/api/v1/backups/:id/download", handler.Download)

	return &backupFlowEnv{
		db:      db,
		svcCtx:  svcCtx,
		service: service,
		handler: handler,
		router:  router,
	}
}

var backupExtensionInstallSeq int

func installBackupAdvancedExtension(t *testing.T, env *backupFlowEnv, config map[string]any) *model.ExtensionInstallation {
	t.Helper()
	backupExtensionInstallSeq++
	item, err := env.svcCtx.Extensions.Installation.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    "official.backup-advanced",
		ReleaseVersion: fmt.Sprintf("1.0.%d", backupExtensionInstallSeq),
		ScopeType:      "global",
		ScopeID:        "global",
		TargetType:     "global",
		Config:         config,
		Operator:       "tester",
	})
	require.NoError(t, err)
	return item
}

func createBackupRow(t *testing.T, env *backupFlowEnv, backup *model.Backup) *model.Backup {
	t.Helper()
	require.NoError(t, env.svcCtx.BackupModel.Create(context.Background(), backup))
	return backup
}

// ---------------------------------------------------------------------------
// Service: List
// ---------------------------------------------------------------------------

func TestService_List_FromDatabase(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	createBackupRow(t, env, &model.Backup{BackupID: "bkp-1", Name: "one", Type: "database", Status: "completed"})
	createBackupRow(t, env, &model.Backup{BackupID: "bkp-2", Name: "two", Type: "database", Status: "completed"})
	createBackupRow(t, env, &model.Backup{BackupID: "bkp-3", Name: "three", Type: "config", Status: "completed"})

	resp, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(3), resp.Total)
	assert.Len(t, resp.Items, 3)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.Size)

	resp, err = env.service.List(ctx, &BackupsListRequest{Type: " database "})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Total)

	resp, err = env.service.List(ctx, &BackupsListRequest{Type: "missing"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.Total)
	assert.Empty(t, resp.Items)

	resp, err = env.service.List(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), resp.Total)
}

func TestService_List_FromExtensionInstallation(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	records := []Backup{
		{Id: "ext-1", Name: "ext-one", Type: "full", Status: "completed"},
		{Id: "ext-2", Name: "ext-two", Type: "incremental", Status: "completed"},
	}
	installBackupAdvancedExtension(t, env, map[string]any{"backups": records})

	// DB rows should be ignored when the extension records exist.
	createBackupRow(t, env, &model.Backup{BackupID: "db-1", Name: "db-one", Type: "full"})

	resp, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Total)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, "ext-1", resp.Items[0].Id)

	resp, err = env.service.List(ctx, &BackupsListRequest{Type: "INCREMENTAL"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "ext-2", resp.Items[0].Id)

	resp, err = env.service.List(ctx, &BackupsListRequest{Page: 2, PageSize: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Total)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "ext-2", resp.Items[0].Id)
}

func TestService_List_ExtensionWithoutRecordsFallsBackToDB(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	installBackupAdvancedExtension(t, env, map[string]any{})
	createBackupRow(t, env, &model.Backup{BackupID: "db-1", Name: "db-one", Type: "full"})

	resp, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "db-1", resp.Items[0].Id)

	// backups: null also falls back
	item := installBackupAdvancedExtension(t, env, nil)
	require.NoError(t, env.db.Exec("UPDATE extension_installations SET config_json = ? WHERE id = ?",
		`{"backups":null}`, item.ID).Error)

	resp, err = env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
}

func TestService_List_ExtensionInvalidConfigJSON(t *testing.T) {
	env := setupBackupFlowEnv(t)

	item := installBackupAdvancedExtension(t, env, nil)
	require.NoError(t, env.db.Exec("UPDATE extension_installations SET config_json = ? WHERE id = ?",
		`{invalid-json`, item.ID).Error)

	_, err := env.service.List(env.authCtx(t), &BackupsListRequest{})
	require.Error(t, err)
}

func TestService_List_ExtensionRecordsWrongType(t *testing.T) {
	env := setupBackupFlowEnv(t)

	installBackupAdvancedExtension(t, env, map[string]any{"backups": "not-an-array"})

	_, err := env.service.List(env.authCtx(t), &BackupsListRequest{})
	require.Error(t, err)
}

func TestService_List_DatabaseError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.Migrator().DropTable("backups"))

	seedBackupSuperAdmin(t, db)
	service := NewService(&svc.ServiceContext{
		DB:          db,
		AdminModel:  model.NewAdminModel(db),
		RoleModel:   model.NewRoleModel(db),
		BackupModel: model.NewBackupModel(db),
	})
	_, err = service.List(context.WithValue(context.Background(), "username", backupSuperAdminUsername), &BackupsListRequest{})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Service: Create
// ---------------------------------------------------------------------------

func TestService_Create_Success(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	resp, err := env.service.Create(ctx, &BackupCreateRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Id)
	assert.Equal(t, "full", resp.Type)
	assert.Equal(t, "pending", resp.Status)
	assert.Contains(t, resp.Name, "full-")

	resp, err = env.service.Create(ctx, &BackupCreateRequest{Name: " weekly ", Type: " INCREMENTAL "})
	require.NoError(t, err)
	assert.Equal(t, "weekly", resp.Name)
	assert.Equal(t, "incremental", resp.Type)

	found, err := env.svcCtx.BackupModel.FindByBackupID(ctx, resp.Id)
	require.NoError(t, err)
	assert.Equal(t, "weekly", found.Name)
}

func TestService_Create_SyncsExtensionRecords(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	installBackupAdvancedExtension(t, env, map[string]any{})

	first, err := env.service.Create(ctx, &BackupCreateRequest{Name: "first", Type: "full"})
	require.NoError(t, err)

	second, err := env.service.Create(ctx, &BackupCreateRequest{Name: "second", Type: "full"})
	require.NoError(t, err)

	resp, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	ids := []string{resp.Items[0].Id, resp.Items[1].Id}
	assert.Contains(t, ids, first.Id)
	assert.Contains(t, ids, second.Id)

	// Upserting an existing id updates in place instead of appending.
	updated := first.Backup
	updated.Name = "first-renamed"
	require.NoError(t, env.service.upsertBackupToExtension(ctx, updated))

	resp, err = env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	var renamed bool
	for _, item := range resp.Items {
		if item.Id == first.Id && item.Name == "first-renamed" {
			renamed = true
		}
	}
	assert.True(t, renamed)
}

func TestService_Create_DatabaseError(t *testing.T) {
	// AUD-082: the env must be executor-capable, otherwise the create-time gate
	// would reject before the database insert is reached and this case would
	// silently stop covering the insert error path.
	src := filepath.Join(t.TempDir(), "create-err-src.db")
	writeRealSQLiteDB(t, src)
	env := newExecTestEnv(t, "sqlite", src, &execObjStoreStub{})
	require.NoError(t, env.db.Migrator().DropTable("backups"))

	_, err := env.service.Create(env.authCtx(t), &BackupCreateRequest{Name: "x"})
	require.Error(t, err)
}

func TestUpsertBackupToExtension_EmptyID(t *testing.T) {
	env := setupBackupFlowEnv(t)
	installBackupAdvancedExtension(t, env, map[string]any{})
	require.NoError(t, env.service.upsertBackupToExtension(context.Background(), Backup{Name: "no-id"}))
}

// ---------------------------------------------------------------------------
// Service: Delete
// ---------------------------------------------------------------------------

func TestService_Delete_Success(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	row := createBackupRow(t, env, &model.Backup{BackupID: "bkp-del", Name: "del", Type: "full"})

	require.NoError(t, env.service.Delete(ctx, &BackupDeleteRequest{ID: " bkp-del "}))

	_, err := env.svcCtx.BackupModel.FindByBackupID(ctx, row.BackupID)
	require.Error(t, err)
}

func TestService_Delete_RemovesExtensionRecord(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	installBackupAdvancedExtension(t, env, map[string]any{"backups": []Backup{
		{Id: "ext-keep", Type: "full"},
		{Id: "ext-drop", Type: "full"},
	}})

	createBackupRow(t, env, &model.Backup{BackupID: "ext-drop", Name: "drop", Type: "full"})
	require.NoError(t, env.service.Delete(ctx, &BackupDeleteRequest{ID: "ext-drop"}))

	resp, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "ext-keep", resp.Items[0].Id)
}

func TestService_Delete_EmptyID(t *testing.T) {
	env := setupBackupFlowEnv(t)
	err := env.service.Delete(env.authCtx(t), &BackupDeleteRequest{ID: "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "备份ID不能为空")
}

func TestService_Delete_NotFound(t *testing.T) {
	env := setupBackupFlowEnv(t)
	err := env.service.Delete(env.authCtx(t), &BackupDeleteRequest{ID: "missing"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)
}

func TestService_Delete_DatabaseError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.Migrator().DropTable("backups"))

	seedBackupSuperAdmin(t, db)
	service := NewService(&svc.ServiceContext{
		DB:          db,
		AdminModel:  model.NewAdminModel(db),
		RoleModel:   model.NewRoleModel(db),
		BackupModel: model.NewBackupModel(db),
	})
	err = service.Delete(context.WithValue(context.Background(), "username", backupSuperAdminUsername), &BackupDeleteRequest{ID: "any"})
	require.Error(t, err)
}

func TestRemoveBackupFromExtension_NoRecords(t *testing.T) {
	env := setupBackupFlowEnv(t)

	// No installation at all.
	require.NoError(t, env.service.removeBackupFromExtension(context.Background(), "bkp-x"))

	// Installation without records.
	installBackupAdvancedExtension(t, env, map[string]any{})
	require.NoError(t, env.service.removeBackupFromExtension(context.Background(), "bkp-x"))

	// Empty id short-circuits.
	require.NoError(t, env.service.removeBackupFromExtension(context.Background(), "  "))
}

// ---------------------------------------------------------------------------
// Service: Download
// ---------------------------------------------------------------------------

func TestService_Download_SuccessFlow(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	// After AUD-COV-001 P1, download goes through objstore: the object is written to
	// the base directory via the file driver, recording location = object key
	// (consistent with the backup executor's write semantics).
	base := t.TempDir()
	store, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: base})
	require.NoError(t, err)
	env.svcCtx.ObjectStore = store

	content := "hello-backup"
	key := "backups/nightly.tar"
	require.NoError(t, store.Put(ctx, key, strings.NewReader(content), int64(len(content)), "application/octet-stream"))

	row := createBackupRow(t, env, &model.Backup{
		BackupID: "bkp-dl",
		Name:     "nightly",
		Type:     "full",
		Status:   "completed",
		Location: key,
	})

	payload, err := env.service.Download(ctx, &BackupDownloadRequest{ID: row.BackupID})
	require.NoError(t, err)
	require.NotNil(t, payload)
	assert.Empty(t, payload.RedirectURL)
	assert.Equal(t, "nightly", payload.Filename)
	assert.Equal(t, int64(len(content)), payload.Size)
	require.NotNil(t, payload.Reader)
	defer func() {
		_ = payload.Reader.(interface {
			Close() error
		}).Close()
	}()
}

// AUD-COV-001 P1: file:// and absolute/out-of-bounds paths no longer read the local
// filesystem directly (the arbitrary-file read surface has converged onto objstore
// semantics) and are rejected outright.
func TestService_Download_RejectsLocalFilesystemPaths(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	base := t.TempDir()
	store, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: base})
	require.NoError(t, err)
	env.svcCtx.ObjectStore = store

	dir := t.TempDir()
	filePath := filepath.Join(dir, "secret.tar")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0o600))

	for _, tc := range []struct {
		name     string
		location string
	}{
		{"absolute", filePath},
		{"file-uri", "file://" + filePath},
		{"traversal", "../../" + filepath.Base(dir) + "/" + filepath.Base(filePath)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := createBackupRow(t, env, &model.Backup{
				BackupID: "bkp-rej-" + tc.name,
				Name:     tc.name,
				Location: tc.location,
			})
			_, err := env.service.Download(ctx, &BackupDownloadRequest{ID: row.BackupID})
			require.Error(t, err, "location %q 不得直读本地路径", tc.location)
		})
	}
}

// The old file:// direct-read case was replaced by
// TestService_Download_RejectsLocalFilesystemPaths (AUD-COV-001 P1: file:// no longer
// maps local paths).

func TestService_Download_RelativePath(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	// Relative key semantics converge onto objstore: resolved against the base
	// directory, no longer against the process CWD.
	store, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	env.svcCtx.ObjectStore = store

	require.NoError(t, store.Put(ctx, "relative.tar", strings.NewReader("rel"), 3, "application/octet-stream"))

	createBackupRow(t, env, &model.Backup{
		BackupID: "bkp-rel",
		Name:     "rel-backup",
		Location: "relative.tar",
	})

	payload, err := env.service.Download(ctx, &BackupDownloadRequest{ID: "bkp-rel"})
	require.NoError(t, err)
	defer func() {
		_ = payload.Reader.(interface {
			Close() error
		}).Close()
	}()
	assert.Equal(t, int64(3), payload.Size)
}

// listOkGetFailStub: List reports the object exists but Get fails, covering the
// "failed to open object" branch (the old implementation's unix socket open ENXIO
// scenario is unreachable under objstore semantics, replaced by injection).
type listOkGetFailStub struct{}

func (s *listOkGetFailStub) Put(ctx context.Context, key string, r objstore.ReadSeeker, size int64, contentType string) error {
	return errors.New("not implemented")
}
func (s *listOkGetFailStub) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, errors.New("forced get failure")
}
func (s *listOkGetFailStub) SignedURL(ctx context.Context, key string, method string, expiry time.Duration) (string, error) {
	return "", errors.New("not implemented")
}
func (s *listOkGetFailStub) Delete(ctx context.Context, key string) error {
	return errors.New("not implemented")
}
func (s *listOkGetFailStub) List(ctx context.Context, prefix, marker, delimiter string, limit int) (objstore.ListResult, error) {
	return objstore.ListResult{Objects: []objstore.ObjectInfo{{Key: "backups/stuck.tar", Size: 12}}}, nil
}
func (s *listOkGetFailStub) CreatePrefix(ctx context.Context, prefix string) error {
	return errors.New("not implemented")
}
func (s *listOkGetFailStub) RenamePrefix(ctx context.Context, oldPrefix, newPrefix string) error {
	return errors.New("not implemented")
}

func TestService_Download_ObjectLookupAndOpenFailures(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	t.Run("对象不存在：List 无精确匹配", func(t *testing.T) {
		store, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: t.TempDir()})
		require.NoError(t, err)
		env.svcCtx.ObjectStore = store

		createBackupRow(t, env, &model.Backup{
			BackupID: "bkp-missing",
			Name:     "missing",
			Location: "backups/no-such-backup.tar",
		})
		_, err = env.service.Download(ctx, &BackupDownloadRequest{ID: "bkp-missing"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "备份对象不存在")
	})

	t.Run("Get 失败：List 命中但打开失败", func(t *testing.T) {
		env.svcCtx.ObjectStore = &listOkGetFailStub{}

		createBackupRow(t, env, &model.Backup{
			BackupID: "bkp-stuck",
			Name:     "stuck",
			Location: "backups/stuck.tar",
		})
		_, err := env.service.Download(ctx, &BackupDownloadRequest{ID: "bkp-stuck"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "无法打开备份文件")
	})
}

func TestService_Download_RemoteRedirect(t *testing.T) {
	env := setupBackupFlowEnv(t)

	createBackupRow(t, env, &model.Backup{
		BackupID: "bkp-remote",
		Location: "https://storage.example.com/backups/bkp-remote.tar",
	})

	payload, err := env.service.Download(env.authCtx(t), &BackupDownloadRequest{ID: "bkp-remote"})
	require.NoError(t, err)
	assert.Equal(t, "https://storage.example.com/backups/bkp-remote.tar", payload.RedirectURL)
	assert.Nil(t, payload.Reader)
}

func TestService_Download_EmptyID(t *testing.T) {
	env := setupBackupFlowEnv(t)
	_, err := env.service.Download(env.authCtx(t), &BackupDownloadRequest{ID: " "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "备份ID不能为空")
}

func TestService_Download_NotFound(t *testing.T) {
	env := setupBackupFlowEnv(t)
	_, err := env.service.Download(env.authCtx(t), &BackupDownloadRequest{ID: "nope"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)
}

func TestService_Download_NoLocation(t *testing.T) {
	env := setupBackupFlowEnv(t)

	createBackupRow(t, env, &model.Backup{BackupID: "bkp-empty", Name: "pending-backup"})

	_, err := env.service.Download(env.authCtx(t), &BackupDownloadRequest{ID: "bkp-empty"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
}

func TestService_Download_FileMissing(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	store, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	env.svcCtx.ObjectStore = store

	createBackupRow(t, env, &model.Backup{
		BackupID: "bkp-gone",
		Name:     "gone",
		Location: "backups/does-not-exist.tar",
	})

	_, err = env.service.Download(ctx, &BackupDownloadRequest{ID: "bkp-gone"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)
}

// ---------------------------------------------------------------------------
// Service: extension installation helpers
// ---------------------------------------------------------------------------

func TestFindActiveBackupInstallation(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	// No installations.
	_, ok, err := env.service.findActiveBackupInstallation(ctx)
	require.NoError(t, err)
	assert.False(t, ok)

	// All uninstalled -> skipped.
	uninstalled := installBackupAdvancedExtension(t, env, nil)
	require.NoError(t, env.svcCtx.Extensions.Installation.Uninstall(ctx, uninstalled.ID, "tester"))
	_, ok, err = env.service.findActiveBackupInstallation(ctx)
	require.NoError(t, err)
	assert.False(t, ok)

	// DesiredState uninstalled -> skipped.
	desiredOnly := installBackupAdvancedExtension(t, env, nil)
	require.NoError(t, env.db.Exec("UPDATE extension_installations SET desired_state = ? WHERE id = ?",
		"uninstalled", desiredOnly.ID).Error)
	_, ok, err = env.service.findActiveBackupInstallation(ctx)
	require.NoError(t, err)
	assert.False(t, ok)

	// Active installation wins.
	active := installBackupAdvancedExtension(t, env, nil)
	item, ok, err := env.service.findActiveBackupInstallation(ctx)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, active.ID, item.ID)
}

func TestFindActiveBackupInstallation_NilDeps(t *testing.T) {
	service := NewService(nil)
	_, ok, err := service.findActiveBackupInstallation(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)

	service = NewService(&svc.ServiceContext{})
	_, ok, err = service.findActiveBackupInstallation(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)

	service = NewService(&svc.ServiceContext{Extensions: &svc.ExtensionServices{}})
	_, ok, err = service.findActiveBackupInstallation(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRecordBackupEvent(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := context.WithValue(context.Background(), "username", "alice")

	// Without installation -> nil error.
	require.NoError(t, env.service.recordBackupEvent(ctx, "backups_create", "backup created", "{}"))

	item := installBackupAdvancedExtension(t, env, nil)
	require.NoError(t, env.service.recordBackupEvent(ctx, "backups_create", "backup created", `{"backup_id":"bkp-1"}`))

	events, total, err := env.svcCtx.Extensions.Installation.ListEvents(ctx, item.ID, extensioninstallation.EventListQuery{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	found := false
	for _, evt := range events {
		if evt.EventType == "backups_create" && evt.CreatedBy == "alice" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestSaveBackupsToExtensionInstallation_PreservesSecretRefs(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	item := installBackupAdvancedExtension(t, env, map[string]any{"other": "value"})
	require.NoError(t, env.db.Exec("UPDATE extension_installations SET secret_refs_json = ? WHERE id = ?",
		`{"token":"vault://backup"}`, item.ID).Error)

	require.NoError(t, env.service.saveBackupsToExtensionInstallation(ctx, []Backup{{Id: "bkp-1"}}))

	var updated model.ExtensionInstallation
	require.NoError(t, env.db.First(&updated, item.ID).Error)
	assert.Contains(t, string(updated.ConfigJSON), "bkp-1")
	assert.Contains(t, string(updated.ConfigJSON), "other")
	assert.Contains(t, string(updated.SecretRefsJSON), "vault://backup")
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

func doBackupRequest(t *testing.T, env *backupFlowEnv, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return doBackupRequestAs(t, env, method, path, body, backupSuperAdminUsername)
}

// doBackupRequestAs issues a backup route request as the given username (simulating
// the AuthMiddleware contract of placing username in the request context), driving
// both the allow and deny sides of the service-layer authorization gate.
func doBackupRequestAs(t *testing.T, env *backupFlowEnv, method, path, body, username string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req = req.WithContext(context.WithValue(req.Context(), "username", username))
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func TestHandler_List(t *testing.T) {
	env := setupBackupFlowEnv(t)

	createBackupRow(t, env, &model.Backup{BackupID: "bkp-h1", Name: "handler-one", Type: "full"})

	rec := doBackupRequest(t, env, http.MethodGet, "/api/v1/backups?page=1&pageSize=10", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "bkp-h1")

	rec = doBackupRequest(t, env, http.MethodGet, "/api/v1/backups?page=abc", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "page=abc 数值转换错误应返回 400（Bug4 修复）")
}

func TestHandler_Create(t *testing.T) {
	env := setupBackupFlowEnv(t)

	rec := doBackupRequest(t, env, http.MethodPost, "/api/v1/backups", `{"name":"api-backup","type":"full"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "api-backup")

	rec = doBackupRequest(t, env, http.MethodPost, "/api/v1/backups", `{`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Delete(t *testing.T) {
	env := setupBackupFlowEnv(t)

	createBackupRow(t, env, &model.Backup{BackupID: "bkp-hdel", Name: "delete-me", Type: "full"})

	rec := doBackupRequest(t, env, http.MethodDelete, "/api/v1/backups/bkp-hdel", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "操作成功")

	rec = doBackupRequest(t, env, http.MethodDelete, "/api/v1/backups/missing-id", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Download(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	// After AUD-COV-001 P1: handler download goes through objstore (recording
	// location = object key).
	store, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	env.svcCtx.ObjectStore = store

	content := "payload-data"
	require.NoError(t, store.Put(ctx, "backups/download.tar", strings.NewReader(content), int64(len(content)), "application/octet-stream"))

	createBackupRow(t, env, &model.Backup{
		BackupID: "bkp-hdl",
		Name:     "download-me",
		Location: "backups/download.tar",
	})
	createBackupRow(t, env, &model.Backup{
		BackupID: "bkp-hremote",
		Location: "https://storage.example.com/bkp-hremote.tar",
	})

	rec := doBackupRequest(t, env, http.MethodGet, "/api/v1/backups/bkp-hdl/download", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "attachment; filename=download-me", rec.Header().Get("Content-Disposition"))
	assert.Equal(t, "payload-data", rec.Body.String())

	rec = doBackupRequest(t, env, http.MethodGet, "/api/v1/backups/bkp-hremote/download", "")
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "https://storage.example.com/bkp-hremote.tar", rec.Header().Get("Location"))

	rec = doBackupRequest(t, env, http.MethodGet, "/api/v1/backups/unknown/download", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_ServiceErrors(t *testing.T) {
	env := setupBackupFlowEnv(t)

	require.NoError(t, env.db.Migrator().DropTable("backups"))
	defer func() {
		require.NoError(t, env.db.Migrator().CreateTable(&model.Backup{}))
	}()

	rec := doBackupRequest(t, env, http.MethodGet, "/api/v1/backups", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	rec = doBackupRequest(t, env, http.MethodPost, "/api/v1/backups", `{"name":"x"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	rec = doBackupRequest(t, env, http.MethodDelete, "/api/v1/backups/whatever", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	rec = doBackupRequest(t, env, http.MethodGet, "/api/v1/backups/whatever/download", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestService_Download_FileURIInvalid(t *testing.T) {
	env := setupBackupFlowEnv(t)

	base := t.TempDir()
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: base})
	require.NoError(t, err)
	env.svcCtx.ObjectStore = store

	// file:// no longer maps local paths: a control-character URI converges onto
	// "object not found" (fail-closed)
	createBackupRow(t, env, &model.Backup{
		BackupID: "bkp-bad-uri",
		Location: "file://\x7f/backup.tar",
	})

	_, err = env.service.Download(env.authCtx(t), &BackupDownloadRequest{ID: "bkp-bad-uri"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)
}
