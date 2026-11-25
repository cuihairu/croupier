// AUD-016 regression: permission gating of the generic object interface and the
// protected-prefix boundary.
//
// Three kinds of accounts are seeded on a shared sqlite: a super admin
// (HasAdminRole passes straight through, used as the control), an operator
// holding only uploads:write (proving that the existing permission definition
// alone grants access), and a zero-role account (proving that the old
// "any logged-in user could overwrite shared objects" hole is closed).
package storage

import (
	"context"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// storageSharedDB is a package-level shared sqlite (same paradigm as the backup
// package): the Admin/Role tables and seed accounts are migrated/seeded once on
// first use and reused by every case in the package, avoiding a ~100-table
// AutoMigrate cost per case.
var (
	storageSharedDBOnce sync.Once
	storageSharedDBPtr  *gorm.DB
)

const (
	// storageSuperAdminUsername is the super admin account: HasAdminRole passes
	// straight through; it is the control for the data plane.
	storageSuperAdminUsername = "storage_superadmin"
	// storageOperatorUsername is an operator holding only uploads:write: proves that
	// this existing permission is enough to perform storage operations (the data
	// plane works and rejections are not "resource missing").
	storageOperatorUsername = "storage_operator"
	// storageViewerUsername is a zero-role account: the login context is fine but
	// there are no roles/permissions at all, so RequireAnyPermission must deny.
	storageViewerUsername = "storage_viewer"
)

func storageSharedDB(t *testing.T) *gorm.DB {
	t.Helper()
	storageSharedDBOnce.Do(func() {
		db, err := gorm.Open(sqlite.Open("file:storageshared?mode=memory&cache=shared"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		require.NoError(t, err)
		require.NoError(t, model.AutoMigrate(db))
		seedStorageSuperAdmin(t, db)
		seedStorageOperator(t, db)
		seedStorageViewer(t, db)
		storageSharedDBPtr = db
	})
	return storageSharedDBPtr
}

// seedStorageSuperAdmin idempotently creates the super_admin account (the control).
func seedStorageSuperAdmin(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin, err := adminModel.FindByUsername(ctx, storageSuperAdminUsername)
	if err != nil {
		admin = &model.Admin{Username: storageSuperAdminUsername, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}

	hasAdmin := false
	roles, err := adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "super_admin" {
			hasAdmin = true
			break
		}
	}
	if !hasAdmin {
		role := &model.Role{Name: "super_admin"}
		require.NoError(t, roleModel.Create(ctx, role))
		require.NoError(t, adminModel.AssignRole(ctx, admin.ID, role.ID))
	}
}

// seedStorageOperator idempotently creates an operator account holding only
// uploads:write.
func seedStorageOperator(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin, err := adminModel.FindByUsername(ctx, storageOperatorUsername)
	if err != nil {
		admin = &model.Admin{Username: storageOperatorUsername, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}

	hasRole := false
	roles, err := adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "storage_operator" {
			hasRole = true
			break
		}
	}
	if !hasRole {
		role := &model.Role{Name: "storage_operator"}
		require.NoError(t, roleModel.Create(ctx, role))
		require.NoError(t, adminModel.AssignRole(ctx, admin.ID, role.ID))
	}
	// Re-attach the uploads:write permission on every seeding pass (idempotent),
	// proving "holding this permission is enough to operate".
	roles, err = adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "storage_operator" {
			require.NoError(t, roleModel.ReplacePermissions(ctx, r.ID, []string{storagePermission}))
			break
		}
	}
}

// seedStorageViewer idempotently creates a zero-role account (no roles/permissions at all).
func seedStorageViewer(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)

	admin, err := adminModel.FindByUsername(ctx, storageViewerUsername)
	if err != nil {
		admin = &model.Admin{Username: storageViewerUsername, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}
}

// identitySvcCtx returns a svcCtx carrying the seeded accounts' identity surface
// (Admin/Role/DB) plus the given object store: existing cases that build their own
// svcCtx upgrade through this uniformly, so the service-level permission gate can
// be exercised.
func identitySvcCtx(t *testing.T, store objstore.Store) *svc.ServiceContext {
	t.Helper()
	db := storageSharedDB(t)
	return &svc.ServiceContext{
		DB:          db,
		AdminModel:  model.NewAdminModel(db),
		RoleModel:   model.NewRoleModel(db),
		ObjectStore: store,
	}
}

// authCtx returns a request context carrying the super admin username (the control).
func authCtx() context.Context {
	return context.WithValue(context.Background(), "username", storageSuperAdminUsername)
}

// operatorCtx returns a request context carrying the uploads:write-only operator.
func operatorCtx() context.Context {
	return context.WithValue(context.Background(), "username", storageOperatorUsername)
}

// viewerCtx returns a request context carrying the zero-role account.
func viewerCtx() context.Context {
	return context.WithValue(context.Background(), "username", storageViewerUsername)
}
