// AUD-016 回归：通用对象接口的权限门禁与受保护前缀边界。
//
// 共享 sqlite 上播种三类账号：超管（HasAdminRole 直通，作为对照）、仅持
// uploads:write 的操作员（证明既有权限定义即放行）、零角色账号（证明此前
// 「仅登录即可覆盖共享对象」的洞已关闭）。
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

// storageSharedDB 是包级共享 sqlite（同 backup 包范式）：Admin/Role 表与种子
// 账号在首次调用时迁移/播种一次，包内各用例复用，避免每用例 ~100 张表的
// AutoMigrate 成本。
var (
	storageSharedDBOnce sync.Once
	storageSharedDBPtr  *gorm.DB
)

const (
	// storageSuperAdminUsername 超管账号：HasAdminRole 直通，对照数据面可用。
	storageSuperAdminUsername = "storage_superadmin"
	// storageOperatorUsername 仅持 uploads:write 的操作员：证明该既有权限
	// 即可执行存储操作（数据面正常、拒绝非「资源缺失」）。
	storageOperatorUsername = "storage_operator"
	// storageViewerUsername 零角色账号：登录上下文正常但无任何角色/权限，
	// RequireAnyPermission 必须拒绝。
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

// seedStorageSuperAdmin 幂等创建 super_admin 超管账号（对照）。
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

// seedStorageOperator 幂等创建仅持 uploads:write 权限的操作员账号。
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
	// 每次播种都重挂 uploads:write 权限（幂等），证明「持该权限即可操作」。
	roles, err = adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "storage_operator" {
			require.NoError(t, roleModel.ReplacePermissions(ctx, r.ID, []string{storagePermission}))
			break
		}
	}
}

// seedStorageViewer 幂等创建零角色账号（无任何角色/权限）。
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

// identitySvcCtx 返回带种子账号身份面（Admin/Role/DB）+ 指定对象存储的
// svcCtx：既有用例自建 svcCtx 时统一经此升级，让服务层权限门禁可过。
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

// authCtx 返回携带超管用户名的请求上下文（对照）。
func authCtx() context.Context {
	return context.WithValue(context.Background(), "username", storageSuperAdminUsername)
}

// operatorCtx 返回携带仅持 uploads:write 操作员的请求上下文。
func operatorCtx() context.Context {
	return context.WithValue(context.Background(), "username", storageOperatorUsername)
}

// viewerCtx 返回携带零角色账号的请求上下文。
func viewerCtx() context.Context {
	return context.WithValue(context.Background(), "username", storageViewerUsername)
}
