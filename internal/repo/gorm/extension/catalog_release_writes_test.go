package extensiongorm

// catalog/release 写路径与批量读覆盖（覆盖率巡检）：这七个方法此前整段 0%
// ——GetByExtensionIDs/Create/UpdateByExtensionID/DeleteByExtensionID/
// GetByExtensionIDAndVersion/ReleaseRepo.Create/DeleteByExtensionID。
//
// 手法沿用本仓既有批次三口径：读翼 DropTable 缺表；写翼
// `CREATE TRIGGER ... BEFORE UPDATE/DELETE ... RAISE(ABORT)` 打穿
// `res.Error` 分支；RowsAffected == 0 走 gorm.ErrRecordNotFound 语义。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// crwDB 起一个仅含 catalog/release 两表的内存库。
func crwDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:crw?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionCatalog{}, &model.ExtensionRelease{}))
	require.NoError(t, db.Exec("DELETE FROM extension_catalogs").Error)
	require.NoError(t, db.Exec("DELETE FROM extension_releases").Error)
	return db
}

// crwCatalog 造一条目录行并返回其 extension_id。
func crwCatalog(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	require.NoError(t, db.Create(&model.ExtensionCatalog{
		ExtensionID: id, Name: "n-" + id, DisplayName: "d-" + id, Vendor: "v",
		Kind: "source", Status: "active",
	}).Error)
}

// crwRelease 造一条版本行。
func crwRelease(t *testing.T, db *gorm.DB, id, version string) {
	t.Helper()
	require.NoError(t, db.Create(&model.ExtensionRelease{
		ExtensionID: id, Version: version, ReleaseChannel: "stable",
		ManifestJSON: model.JSON(`{"v":1}`),
	}).Error)
}

// ---- CatalogRepo.GetByExtensionIDs ----

func TestCatalogRepo_GetByExtensionIDs(t *testing.T) {
	db := crwDB(t)
	repo := NewCatalogRepo(db)
	ctx := context.Background()
	crwCatalog(t, db, "e1")
	crwCatalog(t, db, "e2")

	t.Run("批量命中", func(t *testing.T) {
		items, err := repo.GetByExtensionIDs(ctx, []string{"e1", "e2", "missing"})
		require.NoError(t, err)
		assert.Len(t, items, 2, "不存在的 id 应直接缺席而非报错")
	})

	t.Run("空入参短路", func(t *testing.T) {
		items, err := repo.GetByExtensionIDs(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, items)
		items, err = repo.GetByExtensionIDs(ctx, []string{})
		require.NoError(t, err)
		assert.Nil(t, items)
	})

	t.Run("缺表报错", func(t *testing.T) {
		broken, err := gorm.Open(gsqlite.Open("file:crw_gbi?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		r := NewCatalogRepo(broken)
		_, err = r.GetByExtensionIDs(ctx, []string{"e1"})
		require.Error(t, err)
	})
}

// ---- CatalogRepo.Create ----

func TestCatalogRepo_Create(t *testing.T) {
	db := crwDB(t)
	repo := NewCatalogRepo(db)
	ctx := context.Background()

	t.Run("写入后可读回", func(t *testing.T) {
		item := &model.ExtensionCatalog{
			ExtensionID: "c1", Name: "C1", DisplayName: "显示", Vendor: "acme",
			Kind: "validator", Status: "active", LatestVersion: "1.0.0",
		}
		require.NoError(t, repo.Create(ctx, item))
		assert.NotZero(t, item.ID)
		got, err := repo.GetByExtensionID(ctx, "c1")
		require.NoError(t, err)
		assert.Equal(t, "显示", got.DisplayName)
		assert.Equal(t, "1.0.0", got.LatestVersion)
	})

	t.Run("唯一键冲突报错", func(t *testing.T) {
		err := repo.Create(ctx, &model.ExtensionCatalog{
			ExtensionID: "c1", Name: "dup", Kind: "source", Status: "active",
		})
		require.Error(t, err)
	})

	t.Run("缺表报错", func(t *testing.T) {
		broken, err := gorm.Open(gsqlite.Open("file:crw_c?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		require.Error(t, NewCatalogRepo(broken).Create(ctx, &model.ExtensionCatalog{
			ExtensionID: "x", Name: "x", Kind: "source", Status: "active",
		}))
	})
}

// ---- CatalogRepo.UpdateByExtensionID ----

func TestCatalogRepo_UpdateByExtensionID(t *testing.T) {
	db := crwDB(t)
	repo := NewCatalogRepo(db)
	ctx := context.Background()
	crwCatalog(t, db, "u1")

	t.Run("按列更新", func(t *testing.T) {
		require.NoError(t, repo.UpdateByExtensionID(ctx, "u1", map[string]any{
			"status": "deprecated", "latest_version": "2.0.0",
		}))
		got, err := repo.GetByExtensionID(ctx, "u1")
		require.NoError(t, err)
		assert.Equal(t, "deprecated", got.Status)
		assert.Equal(t, "2.0.0", got.LatestVersion)
		assert.Equal(t, "n-u1", got.Name, "未列出的列不应被改")
	})

	t.Run("行不存在返回 ErrRecordNotFound", func(t *testing.T) {
		err := repo.UpdateByExtensionID(ctx, "nope", map[string]any{"status": "active"})
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("缺表报错", func(t *testing.T) {
		broken, err := gorm.Open(gsqlite.Open("file:crw_u?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		require.Error(t, NewCatalogRepo(broken).UpdateByExtensionID(ctx, "u1",
			map[string]any{"status": "active"}))
	})

	t.Run("写翼拦 UPDATE 打穿 Error 分支", func(t *testing.T) {
		require.NoError(t, db.Exec(
			`CREATE TRIGGER crw_block_update BEFORE UPDATE ON extension_catalogs
			 BEGIN SELECT RAISE(ABORT, 'blocked'); END`).Error)
		defer func() {
			_ = db.Exec("DROP TRIGGER crw_block_update").Error
		}()
		err := repo.UpdateByExtensionID(ctx, "u1", map[string]any{"status": "active"})
		require.Error(t, err)
		assert.NotErrorIs(t, err, gorm.ErrRecordNotFound, "拦截错误不应退化为未找到")
	})
}

// ---- CatalogRepo.DeleteByExtensionID ----

func TestCatalogRepo_DeleteByExtensionID(t *testing.T) {
	db := crwDB(t)
	repo := NewCatalogRepo(db)
	ctx := context.Background()
	crwCatalog(t, db, "d1")

	t.Run("物理删除后可再登记同 id", func(t *testing.T) {
		require.NoError(t, repo.DeleteByExtensionID(ctx, "d1"))
		_, err := repo.GetByExtensionID(ctx, "d1")
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		// 物理删除（Unscoped）才使 extension_id 唯一索引释放
		crwCatalog(t, db, "d1")
		got, err := repo.GetByExtensionID(ctx, "d1")
		require.NoError(t, err)
		assert.Equal(t, "n-d1", got.Name)
	})

	t.Run("行不存在返回 ErrRecordNotFound", func(t *testing.T) {
		err := repo.DeleteByExtensionID(ctx, "nope")
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("缺表报错", func(t *testing.T) {
		broken, err := gorm.Open(gsqlite.Open("file:crw_d?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		require.Error(t, NewCatalogRepo(broken).DeleteByExtensionID(ctx, "d1"))
	})

	t.Run("写翼拦 DELETE 打穿 Error 分支", func(t *testing.T) {
		require.NoError(t, db.Exec(
			`CREATE TRIGGER crw_block_delete BEFORE DELETE ON extension_catalogs
			 BEGIN SELECT RAISE(ABORT, 'blocked'); END`).Error)
		defer func() {
			_ = db.Exec("DROP TRIGGER crw_block_delete").Error
		}()
		err := repo.DeleteByExtensionID(ctx, "d1")
		require.Error(t, err)
		assert.NotErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

// ---- ReleaseRepo ----

func TestReleaseRepo_GetByExtensionIDAndVersion(t *testing.T) {
	db := crwDB(t)
	repo := NewReleaseRepo(db)
	ctx := context.Background()
	crwCatalog(t, db, "r1")
	crwRelease(t, db, "r1", "1.0.0")
	crwRelease(t, db, "r1", "2.0.0")

	t.Run("命中", func(t *testing.T) {
		got, err := repo.GetByExtensionIDAndVersion(ctx, "r1", "2.0.0")
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", got.Version)
	})

	t.Run("版本不存在报错", func(t *testing.T) {
		_, err := repo.GetByExtensionIDAndVersion(ctx, "r1", "9.9.9")
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("缺表报错", func(t *testing.T) {
		broken, err := gorm.Open(gsqlite.Open("file:crw_r?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		_, err = NewReleaseRepo(broken).GetByExtensionIDAndVersion(ctx, "r1", "1.0.0")
		require.Error(t, err)
	})
}

func TestReleaseRepo_Create(t *testing.T) {
	db := crwDB(t)
	repo := NewReleaseRepo(db)
	ctx := context.Background()

	t.Run("写入后可读回", func(t *testing.T) {
		rel := &model.ExtensionRelease{
			ExtensionID: "n1", Version: "1.0.0", ReleaseChannel: "beta",
			ManifestJSON: model.JSON(`{"a":1}`), PackageRef: "sha256:abc",
			Checksum: "sha256:abc", PublishedAtUnix: 1727500000,
		}
		require.NoError(t, repo.Create(ctx, rel))
		assert.NotZero(t, rel.ID)
		got, err := repo.GetByExtensionIDAndVersion(ctx, "n1", "1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "sha256:abc", got.Checksum)
	})

	t.Run("repo 层不校验版本唯一性（caller 职责）", func(t *testing.T) {
		// idx_extension_release_version 是普通复合索引而非唯一索引——写重复行
		// 不报错。这是「唯一性校验由调用方承担」的服务层注释的契约锁定：
		// 若哪天改成唯一索引，本用例会失败并提示同步收紧调用方校验。
		require.NoError(t, repo.Create(ctx, &model.ExtensionRelease{
			ExtensionID: "n1", Version: "1.0.0", ReleaseChannel: "stable",
			ManifestJSON: model.JSON(`{}`),
		}))
		got, err := repo.GetByExtensionIDAndVersion(ctx, "n1", "1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "beta", got.ReleaseChannel, "First 按主键序命中最早写入那条")
	})

	t.Run("缺表报错", func(t *testing.T) {
		broken, err := gorm.Open(gsqlite.Open("file:crw_rc?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		require.Error(t, NewReleaseRepo(broken).Create(ctx, &model.ExtensionRelease{
			ExtensionID: "z", Version: "1.0.0", ReleaseChannel: "stable",
			ManifestJSON: model.JSON(`{}`),
		}))
	})
}

func TestReleaseRepo_DeleteByExtensionID(t *testing.T) {
	db := crwDB(t)
	repo := NewReleaseRepo(db)
	ctx := context.Background()
	crwCatalog(t, db, "k1")
	crwRelease(t, db, "k1", "1.0.0")
	crwRelease(t, db, "k1", "2.0.0")
	crwRelease(t, db, "k2", "1.0.0")

	t.Run("只删目标扩展的全部版本", func(t *testing.T) {
		require.NoError(t, repo.DeleteByExtensionID(ctx, "k1"))
		items, err := repo.ListByExtensionID(ctx, "k1")
		require.NoError(t, err)
		assert.Empty(t, items)
		other, err := repo.ListByExtensionID(ctx, "k2")
		require.NoError(t, err)
		assert.Len(t, other, 1, "他扩展版本不应被牵连")
	})

	t.Run("无匹配行不报错（级联语义）", func(t *testing.T) {
		require.NoError(t, repo.DeleteByExtensionID(ctx, "nobody"))
	})

	t.Run("缺表报错", func(t *testing.T) {
		broken, err := gorm.Open(gsqlite.Open("file:crw_rd?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		require.Error(t, NewReleaseRepo(broken).DeleteByExtensionID(ctx, "k1"))
	})
}
