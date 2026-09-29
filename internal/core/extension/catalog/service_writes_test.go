package catalog

// 写路径与版本路径服务层覆盖（覆盖率巡检）：ListByExtensionIDs / Create /
// UpdateFields / Remove / PublishRelease / ReleaseByVersion / RemoveReleases
// 七个方法此前整段 0%（38.2%）。每个方法三形态：nil 接收者 / 缺仓储守卫 /
// 真实库主链，另加仓储层错误透传（缺表、唯一键冲突、ErrRecordNotFound）。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// cwSeed 造一条目录行。
func cwSeed(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	require.NoError(t, db.Create(&model.ExtensionCatalog{
		ExtensionID: id, Name: "n-" + id, DisplayName: "d-" + id, Vendor: "v",
		Kind: "source", Status: "active",
	}).Error)
}

// cwNoRepos 返回三个「仓储缺失」形态的 Service：全空 / 只有 catalog / 只有
// release。守卫分支按方法依赖的仓储分别校验。
func cwNoRepos() (empty, onlyCatalog, onlyRelease *Service) {
	empty = &Service{}
	onlyCatalog = NewService(extensiongorm.NewCatalogRepo(nil), nil)
	onlyRelease = NewService(nil, extensiongorm.NewReleaseRepo(nil))
	return
}

// TestCatalogService_NilReceiver 七方法对 nil 接收者的守卫语义：读路径回空
// 形态、写路径回 gorm.ErrInvalidDB（方法内先判 s == nil 再判仓储）。
func TestCatalogService_NilReceiver(t *testing.T) {
	var s *Service
	ctx := context.Background()

	items, err := s.ListByExtensionIDs(ctx, []string{"e1"})
	require.NoError(t, err)
	assert.Nil(t, items)
	require.ErrorIs(t, s.Create(ctx, &model.ExtensionCatalog{}), gorm.ErrInvalidDB)
	require.ErrorIs(t, s.UpdateFields(ctx, "e1", map[string]any{"status": "active"}), gorm.ErrInvalidDB)
	require.ErrorIs(t, s.Remove(ctx, "e1"), gorm.ErrInvalidDB)
	require.ErrorIs(t, s.PublishRelease(ctx, &model.ExtensionRelease{}), gorm.ErrInvalidDB)
	_, err = s.ReleaseByVersion(ctx, "e1", "1.0.0")
	require.ErrorIs(t, err, gorm.ErrInvalidDB)
	require.ErrorIs(t, s.RemoveReleases(ctx, "e1"), gorm.ErrInvalidDB)
}

// TestCatalogService_NilRepoGuards 仓储缺失时的守卫：依赖 catalog 仓储的写
// 方法回 ErrInvalidDB，依赖 release 仓储的方法同样回 ErrInvalidDB，而
// ListByExtensionIDs 按读路径约定回 nil,nil。
func TestCatalogService_NilRepoGuards(t *testing.T) {
	ctx := context.Background()
	empty, onlyCatalog, onlyRelease := cwNoRepos()

	for name, s := range map[string]*Service{"empty": empty, "onlyRelease": onlyRelease} {
		require.ErrorIs(t, s.Create(ctx, &model.ExtensionCatalog{}), gorm.ErrInvalidDB, name)
		require.ErrorIs(t, s.UpdateFields(ctx, "e1", map[string]any{"status": "x"}), gorm.ErrInvalidDB, name)
		require.ErrorIs(t, s.Remove(ctx, "e1"), gorm.ErrInvalidDB, name)
		items, err := s.ListByExtensionIDs(ctx, []string{"e1"})
		require.NoError(t, err, name)
		assert.Nil(t, items, name)
	}
	for name, s := range map[string]*Service{"empty": empty, "onlyCatalog": onlyCatalog} {
		require.ErrorIs(t, s.PublishRelease(ctx, &model.ExtensionRelease{}), gorm.ErrInvalidDB, name)
		require.ErrorIs(t, s.RemoveReleases(ctx, "e1"), gorm.ErrInvalidDB, name)
		_, err := s.ReleaseByVersion(ctx, "e1", "1.0.0")
		require.ErrorIs(t, err, gorm.ErrInvalidDB, name)
	}
}

// TestCatalogService_ListByExtensionIDs 批量读主链：命中、缺席、空入参。
func TestCatalogService_ListByExtensionIDs(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()
	cwSeed(t, db, "e1")
	cwSeed(t, db, "e2")

	items, err := svc.ListByExtensionIDs(ctx, []string{"e1", "e2", "ghost"})
	require.NoError(t, err)
	assert.Len(t, items, 2)

	items, err = svc.ListByExtensionIDs(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, items, "空入参短路，不打 DB")

	require.NoError(t, db.Migrator().DropTable(&model.ExtensionCatalog{}))
	_, err = svc.ListByExtensionIDs(ctx, []string{"e1"})
	require.Error(t, err, "仓储错误应透传")
}

// TestCatalogService_Create 主链 + 唯一键冲突 + 缺表。
func TestCatalogService_Create(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()

	item := &model.ExtensionCatalog{
		ExtensionID: "w1", Name: "W1", DisplayName: "显示一", Vendor: "acme",
		Kind: "middleware", Status: "active",
	}
	require.NoError(t, svc.Create(ctx, item))
	assert.NotZero(t, item.ID)

	got, _, err := svc.Get(ctx, "w1")
	require.NoError(t, err)
	assert.Equal(t, "显示一", got.DisplayName)

	require.Error(t, svc.Create(ctx, &model.ExtensionCatalog{
		ExtensionID: "w1", Name: "dup", Kind: "source", Status: "active",
	}), "extension_id 唯一索引应拒绝重复登记")

	require.NoError(t, db.Migrator().DropTable(&model.ExtensionCatalog{}))
	require.Error(t, svc.Create(ctx, &model.ExtensionCatalog{
		ExtensionID: "w2", Name: "W2", Kind: "source", Status: "active",
	}))
}

// TestCatalogService_UpdateFields 主链 + 行不存在 + 缺表。
func TestCatalogService_UpdateFields(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()
	cwSeed(t, db, "u1")

	require.NoError(t, svc.UpdateFields(ctx, "u1", map[string]any{
		"status": "deprecated", "latest_version": "3.1.0",
	}))
	got, _, err := svc.Get(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, "deprecated", got.Status)
	assert.Equal(t, "3.1.0", got.LatestVersion)
	assert.Equal(t, "n-u1", got.Name, "未列出的列不应被改")

	require.ErrorIs(t, svc.UpdateFields(ctx, "ghost", map[string]any{"status": "active"}),
		gorm.ErrRecordNotFound)

	require.NoError(t, db.Migrator().DropTable(&model.ExtensionCatalog{}))
	require.Error(t, svc.UpdateFields(ctx, "u1", map[string]any{"status": "active"}))
}

// TestCatalogService_Remove 主链（物理删除后 id 可再登记）+ 行不存在 + 缺表。
func TestCatalogService_Remove(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()
	cwSeed(t, db, "r1")

	require.NoError(t, svc.Remove(ctx, "r1"))
	_, _, err := svc.Get(ctx, "r1")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 物理删除才释放 extension_id 唯一索引（软删行会继续占用）
	cwSeed(t, db, "r1")
	got, _, err := svc.Get(ctx, "r1")
	require.NoError(t, err)
	assert.Equal(t, "n-r1", got.Name)

	require.ErrorIs(t, svc.Remove(ctx, "ghost"), gorm.ErrRecordNotFound)

	require.NoError(t, db.Migrator().DropTable(&model.ExtensionCatalog{}))
	require.Error(t, svc.Remove(ctx, "r1"))
}

// TestCatalogService_PublishRelease 主链 + 缺表。
func TestCatalogService_PublishRelease(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()
	cwSeed(t, db, "p1")

	rel := &model.ExtensionRelease{
		ExtensionID: "p1", Version: "1.0.0", ReleaseChannel: "stable",
		ManifestJSON: model.JSON(`{"k":"v"}`), PackageRef: "sha256:aa",
	}
	require.NoError(t, svc.PublishRelease(ctx, rel))
	assert.NotZero(t, rel.ID)

	got, err := svc.ReleaseByVersion(ctx, "p1", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "sha256:aa", got.PackageRef)

	require.NoError(t, db.Migrator().DropTable(&model.ExtensionRelease{}))
	require.Error(t, svc.PublishRelease(ctx, &model.ExtensionRelease{
		ExtensionID: "p1", Version: "2.0.0", ReleaseChannel: "stable",
		ManifestJSON: model.JSON(`{}`),
	}))
}

// TestCatalogService_ReleaseByVersion 命中 / 版本不存在 / 缺表。
func TestCatalogService_ReleaseByVersion(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()
	cwSeed(t, db, "v1")
	require.NoError(t, svc.PublishRelease(ctx, &model.ExtensionRelease{
		ExtensionID: "v1", Version: "1.0.0", ReleaseChannel: "stable",
		ManifestJSON: model.JSON(`{}`),
	}))

	got, err := svc.ReleaseByVersion(ctx, "v1", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", got.Version)

	_, err = svc.ReleaseByVersion(ctx, "v1", "9.9.9")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = svc.ReleaseByVersion(ctx, "ghost", "1.0.0")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, db.Migrator().DropTable(&model.ExtensionRelease{}))
	_, err = svc.ReleaseByVersion(ctx, "v1", "1.0.0")
	require.Error(t, err)
}

// TestCatalogService_RemoveReleases 级联清版本：只清目标扩展、清空后再登记
// 同版本号不冲突、缺表报错。
func TestCatalogService_RemoveReleases(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()
	cwSeed(t, db, "x1")
	cwSeed(t, db, "x2")
	for _, v := range []string{"1.0.0", "2.0.0"} {
		require.NoError(t, svc.PublishRelease(ctx, &model.ExtensionRelease{
			ExtensionID: "x1", Version: v, ReleaseChannel: "stable",
			ManifestJSON: model.JSON(`{}`),
		}))
	}
	require.NoError(t, svc.PublishRelease(ctx, &model.ExtensionRelease{
		ExtensionID: "x2", Version: "1.0.0", ReleaseChannel: "stable",
		ManifestJSON: model.JSON(`{}`),
	}))

	require.NoError(t, svc.RemoveReleases(ctx, "x1"))
	_, releases, err := svc.Get(ctx, "x1")
	require.NoError(t, err)
	assert.Empty(t, releases)
	_, other, err := svc.Get(ctx, "x2")
	require.NoError(t, err)
	assert.Len(t, other, 1, "他扩展版本不应被牵连")

	// 清空后可重新发布同版本号（物理删除不留占位）
	require.NoError(t, svc.PublishRelease(ctx, &model.ExtensionRelease{
		ExtensionID: "x1", Version: "1.0.0", ReleaseChannel: "stable",
		ManifestJSON: model.JSON(`{}`),
	}))

	require.NoError(t, db.Migrator().DropTable(&model.ExtensionRelease{}))
	require.Error(t, svc.RemoveReleases(ctx, "x1"))
}
