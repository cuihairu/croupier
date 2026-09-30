package extension

// catalog 写路径（#46 批次 3：登记/改/删/发版）与 pack 导入（批次 6）的错误翼
// 补测：批次 3/6 只补了成功主链，handler.go 五个写端点的错误分支与 service 侧
// 的存储失败翼整段未覆盖（包覆盖停在 94.7%，handler 写段 0%）。
//
// 注入口径沿用本仓既有批次：
//   - 读翼「缺表」：DropTable 后 gorm 立即报错且无副作用，命中「非 NotFound
//     原样透传」那一翼（若误判成 NotFound 会走进重复登记/新建分支，测不出来）；
//   - 写翼「触发器拦写」：BEFORE {INSERT,UPDATE,DELETE} TRIGGER + RAISE(ABORT)，
//     覆盖「校验全过、SQL 真执行才炸」这一类；
//   - 「写成功→复读失败」：BEFORE UPDATE 触发器把行标记软删——UpdateFields 仍
//     报 1 行受影响（仓储按 RowsAffected 判空），紧随其后的复读落 NotFound；
//   - 权限翼：ctx 不带 username，RequireAnyPermission 在 LoadCurrentAdmin 即拦。
//
// 不可达登记：PackImport 第 513 行（json.Marshal(manifestObj) 失败）——入参
// 唯一来源是包内 manifest.json 经 json.Unmarshal 得到的 map[string]any，
// 其值只可能是 string/float64/bool/nil/map/slice，Marshal 无失败形态
// （chan/func/NaN 均无法从 JSON 产生），按房规不造假用例、不删防御分支。

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const cwExtID = "com.example.demo"

// cwDrop 读翼注入：缺表（表名取自 model 的 TableName，不手写）。
func cwDrop(t *testing.T, db *gorm.DB, m any) {
	t.Helper()
	require.NoError(t, db.Migrator().DropTable(m))
}

// cwTrigger 写翼注入：真实执行 SQL 时才炸的拦写触发器。
func cwTrigger(t *testing.T, db *gorm.DB, sql string) {
	t.Helper()
	require.NoError(t, db.Exec(sql).Error)
}

// cwRegister 写路径五端点（路径与 internal/handler/routes.go 注册序一致）。
func cwRegister(env *extensionTestEnv) {
	env.router.POST("/api/v1/extensions/catalog", env.handler.CatalogCreate)
	env.router.PUT("/api/v1/extensions/catalog/:id", env.handler.CatalogUpdate)
	env.router.DELETE("/api/v1/extensions/catalog/:id", env.handler.CatalogDelete)
	env.router.POST("/api/v1/extensions/catalog/:id/releases", env.handler.CatalogReleasePublish)
	env.router.POST("/api/v1/extensions/packs/import", env.handler.PackImport)
}

// cwFailQuery 精确到「某条查询形态」的存储故障注入：catalog Get 内部也会读
// releases，缺表注入到不了「表都在、只有按版本查这一次失败」的形态，故在 gorm
// 查询回调上对命中表名+子串的 SQL 注入错误。
func cwFailQuery(t *testing.T, db *gorm.DB, name, table, substr string) {
	t.Helper()
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == table && strings.Contains(tx.Statement.SQL.String(), substr) {
			tx.AddError(errors.New("cw: injected query failure"))
		}
	}))
}

// cwReadOnlyStore 把对象存储底目录置为只读（Put 的 MkdirAll/Create 必失败）。
// 权限在 TempDir 清理前复原（cleanup 后进先出，本注册晚于 TempDir）。
func cwReadOnlyStore(t *testing.T, env *extensionTestEnv) {
	t.Helper()
	dir := t.TempDir()
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: dir})
	require.NoError(t, err)
	env.svcCtx.ObjectStore = store
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}

// ---- validateCatalogExtensionID ----

func TestValidateCatalogExtensionID_LengthAndLeadingChar(t *testing.T) {
	// 长度上限（129 > 128）。
	err := validateCatalogExtensionID(strings.Repeat("a", 129))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "128")

	// 首字符禁 . _ -（路由参数歧义防护）。
	for _, bad := range []string{".lead", "_lead", "-lead"} {
		err := validateCatalogExtensionID(bad)
		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), "start with", bad)
	}

	// 边界内合法形态不误伤。
	assert.NoError(t, validateCatalogExtensionID(strings.Repeat("a", 128)))
	assert.NoError(t, validateCatalogExtensionID("a.lead-_1"))
}

// ---- CatalogCreate ----

// 登记前的存在性探测拿到非 NotFound 错误时原样透传（不得当成「可新建」）。
func TestService_CatalogCreate_LookupErrorPassthrough(t *testing.T) {
	env := setupExtensionEnv(t)
	cwDrop(t, env.db, &model.ExtensionCatalog{})

	_, err := env.service.CatalogCreate(env.ctx, ExtensionCatalogCreateRequest{ExtensionID: cwExtID}, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such table", "缺表属非 NotFound，应原样透传而非当未登记处理")
}

// 校验全过、INSERT 才炸的拦写翼。
func TestService_CatalogCreate_InsertBlockedByTrigger(t *testing.T) {
	env := setupExtensionEnv(t)
	cwTrigger(t, env.db, `CREATE TRIGGER cw_block_catalog_insert BEFORE INSERT ON extension_catalogs
BEGIN SELECT RAISE(ABORT, 'cw: catalog insert blocked'); END`)

	_, err := env.service.CatalogCreate(env.ctx, ExtensionCatalogCreateRequest{ExtensionID: cwExtID}, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: catalog insert blocked")
}

// ---- CatalogUpdate ----

func TestService_CatalogUpdate_PermissionDenied(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})

	// ctx 不带 username：权限守卫在管理员装载阶段即拦，写路径不得落库。
	_, err := env.service.CatalogUpdate(context.Background(), cwExtID, ExtensionCatalogUpdateRequest{Name: "renamed"})
	require.Error(t, err)

	item, _, err := env.svcCtx.Extensions.Catalog.Get(env.ctx, cwExtID)
	require.NoError(t, err)
	assert.Equal(t, cwExtID, item.Name, "被拒请求不得改写登记行")
}

// name/vendor/kind 三个非空覆盖分支（其余字段无条件写入，与本用例无关）。
func TestService_CatalogUpdate_AppliesNameVendorKind(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})

	resp, err := env.service.CatalogUpdate(env.ctx, cwExtID, ExtensionCatalogUpdateRequest{
		Name:    "renamed",
		Vendor:  "acme",
		Kind:    "integration",
		Summary: "updated summary",
	})
	require.NoError(t, err)
	assert.Equal(t, "renamed", resp.Item.Name)
	assert.Equal(t, "acme", resp.Item.Vendor)
	assert.Equal(t, "integration", resp.Item.Kind)

	item, _, err := env.svcCtx.Extensions.Catalog.Get(env.ctx, cwExtID)
	require.NoError(t, err)
	assert.Equal(t, "renamed", item.Name)
	assert.Equal(t, "acme", item.Vendor)
	assert.Equal(t, "integration", item.Kind)
	assert.Equal(t, "updated summary", item.Summary)
}

func TestService_CatalogUpdate_UpdateBlockedByTrigger(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})
	cwTrigger(t, env.db, `CREATE TRIGGER cw_block_catalog_update BEFORE UPDATE ON extension_catalogs
BEGIN SELECT RAISE(ABORT, 'cw: catalog update blocked'); END`)

	_, err := env.service.CatalogUpdate(env.ctx, cwExtID, ExtensionCatalogUpdateRequest{Name: "renamed"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: catalog update blocked")
}

// 写成功后复读失败翼：BEFORE UPDATE 触发器把行标记软删，UpdateFields 仍算
// 1 行受影响（仓储按 RowsAffected 判空），紧随的复读落 NotFound。
func TestService_CatalogUpdate_RereadAfterSoftDelete(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})
	cwTrigger(t, env.db, `CREATE TRIGGER cw_softdelete_on_update BEFORE UPDATE ON extension_catalogs
BEGIN
  UPDATE extension_catalogs SET deleted_at = CURRENT_TIMESTAMP WHERE extension_id = OLD.extension_id;
END`)

	_, err := env.service.CatalogUpdate(env.ctx, cwExtID, ExtensionCatalogUpdateRequest{Name: "renamed"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource not found", "写成功后的复读应把 NotFound 映射为 404 语义")
}

// ---- CatalogDelete ----

func TestService_CatalogDelete_LookupError(t *testing.T) {
	env := setupExtensionEnv(t)
	cwDrop(t, env.db, &model.ExtensionCatalog{})

	err := env.service.CatalogDelete(env.ctx, cwExtID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such table")
}

// 版本记录清理（RemoveReleases）被拦：登记行必须仍在，不得出现半删状态。
func TestService_CatalogDelete_RemoveReleasesBlocked(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})
	cwTrigger(t, env.db, `CREATE TRIGGER cw_block_release_delete BEFORE DELETE ON extension_releases
BEGIN SELECT RAISE(ABORT, 'cw: release delete blocked'); END`)

	err := env.service.CatalogDelete(env.ctx, cwExtID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: release delete blocked")

	item, _, err := env.svcCtx.Extensions.Catalog.Get(env.ctx, cwExtID)
	require.NoError(t, err, "版本清理失败时登记行不得被删")
	assert.Equal(t, cwExtID, item.ExtensionID)
}

// 活跃实例探测的读翼。
func TestService_RejectActiveInstallations_ListError(t *testing.T) {
	env := setupExtensionEnv(t)
	cwDrop(t, env.db, &model.ExtensionInstallation{})

	err := env.service.rejectActiveInstallations(env.ctx, cwExtID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such table")
}

// ---- CatalogReleasePublish ----

func TestService_CatalogReleasePublish_ReleaseLookupError(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})
	// 只有「按 extension_id+version 查重」这一次失败：前面的 catalog Get
	// （内部读 releases 列表）不受影响，故不能靠缺表注入。
	cwFailQuery(t, env.db, "cw:fail_release_version_lookup", "extension_releases", "version =")

	_, err := env.service.CatalogReleasePublish(env.ctx, cwExtID, ExtensionReleasePublishRequest{
		Version:  "1.1.0",
		Manifest: map[string]any{"capabilities": []any{"cap.echo"}},
	}, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: injected query failure")
}

// 发版写入（PublishRelease）被拦。
func TestService_CatalogReleasePublish_PublishReleaseBlocked(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})
	cwTrigger(t, env.db, `CREATE TRIGGER cw_block_release_insert BEFORE INSERT ON extension_releases
BEGIN SELECT RAISE(ABORT, 'cw: release insert blocked'); END`)

	_, err := env.service.CatalogReleasePublish(env.ctx, cwExtID, ExtensionReleasePublishRequest{
		Version:  "1.1.0",
		Manifest: map[string]any{"capabilities": []any{"cap.echo"}},
	}, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: release insert blocked")
}

// 版本已发布但 latestVersion 回填写入失败：接口须报错，不得静默留下
// 「已发版、指针未回填」的中间态。
func TestService_CatalogReleasePublish_LatestBumpBlocked(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})
	cwTrigger(t, env.db, `CREATE TRIGGER cw_block_catalog_update BEFORE UPDATE ON extension_catalogs
BEGIN SELECT RAISE(ABORT, 'cw: catalog update blocked'); END`)

	_, err := env.service.CatalogReleasePublish(env.ctx, cwExtID, ExtensionReleasePublishRequest{
		Version:  "1.1.0",
		Manifest: map[string]any{"capabilities": []any{"cap.echo"}},
	}, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: catalog update blocked")

	item, releases, err := env.svcCtx.Extensions.Catalog.Get(env.ctx, cwExtID)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", item.LatestVersion, "回填失败不得改写指针")
	assert.Len(t, releases, 2, "版本行已发布（错误发生在其后的指针回填）")
}

// manifest 不可序列化翼：服务层是导出方法，进程内调用方可传入非 JSON 来源的值。
func TestService_CatalogReleasePublish_ManifestMarshalError(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})

	_, err := env.service.CatalogReleasePublish(env.ctx, cwExtID, ExtensionReleasePublishRequest{
		Version:  "1.1.0",
		Manifest: map[string]any{"capabilities": make(chan int)},
	}, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid manifest")
}

// 存量 latestVersion 不可解析时按「未知即回填」处理（不回退指针即失去语义）。
func TestService_CatalogReleasePublish_BumpsUnparsableLatest(t *testing.T) {
	env := setupExtensionEnv(t)
	_, err := env.service.CatalogCreate(env.ctx, ExtensionCatalogCreateRequest{
		ExtensionID:   cwExtID,
		LatestVersion: "nightly-build",
	}, "ext_tester")
	require.NoError(t, err)

	resp, err := env.service.CatalogReleasePublish(env.ctx, cwExtID, ExtensionReleasePublishRequest{
		Version:  "1.1.0",
		Manifest: map[string]any{"capabilities": []any{"cap.echo"}},
	}, "ext_tester")
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", resp.Release.Version)

	item, _, err := env.svcCtx.Extensions.Catalog.Get(env.ctx, cwExtID)
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", item.LatestVersion, "不可解析的存量指针应被首个可解析版本回填")
}

// ---- parseExtensionPack ----

// 同深度第二份 manifest 被跳过（取最先出现的那份，不做覆盖）。
func TestParseExtensionPack_SameDepthLaterSkipped(t *testing.T) {
	files := map[string]string{
		"a/manifest.json": fmt.Sprintf(packManifestTemplate, "com.example.first", "1.0.0", "stable"),
		"b/manifest.json": fmt.Sprintf(packManifestTemplate, "com.example.second", "2.0.0", "beta"),
	}
	m, err := parseExtensionPack(buildPack(t, files))
	require.NoError(t, err)
	assert.Equal(t, "com.example.first", m.ExtensionID, "同深度后到者应被丢弃")
	assert.Equal(t, "1.0.0", m.Version)
}

func TestParseExtensionPack_ManifestTooLarge(t *testing.T) {
	oversize := strings.Repeat("x", extensionPackManifestMaxSize+1)
	_, err := parseExtensionPack(buildPack(t, map[string]string{"manifest.json": oversize}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "大小上限")
}

// 归档中途截断：条目头声明了尺寸但数据不足，读 manifest 失败。
func TestParseExtensionPack_TruncatedManifestRead(t *testing.T) {
	body := fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o644, Size: int64(len(body))}))
	_, err := tw.Write([]byte(body))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	// 只留 512 字节条目头 + 100 字节正文（声明尺寸更大），使条目数据不足。
	const keep = 512 + 100
	require.Greater(t, raw.Len(), keep)
	var gzipped bytes.Buffer
	gw := gzip.NewWriter(&gzipped)
	_, err = gw.Write(raw.Bytes()[:keep])
	require.NoError(t, err)
	require.NoError(t, gw.Close())

	_, err = parseExtensionPack(gzipped.Bytes())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "读取 manifest.json 失败")
}

// ---- PackImport ----

type cwErrReader struct{}

func (cwErrReader) Read([]byte) (int, error) { return 0, errors.New("cw: reader boom") }

// cwZeroReader 产出指定长度的零流（避免真造 64MiB 缓冲）。
type cwZeroReader struct{ remaining int64 }

func (r *cwZeroReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	for i := range p {
		p[i] = 0
	}
	r.remaining -= int64(len(p))
	return len(p), nil
}

func TestService_PackImport_PermissionDenied(t *testing.T) {
	env := newPackEnv(t)
	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")})

	_, err := env.service.PackImport(context.Background(), bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)

	var catalogRows int64
	require.NoError(t, env.db.Model(&model.ExtensionCatalog{}).Count(&catalogRows).Error)
	assert.Zero(t, catalogRows, "被拒导入不得登记 catalog")
	var releaseRows int64
	require.NoError(t, env.db.Model(&model.ExtensionRelease{}).Count(&releaseRows).Error)
	assert.Zero(t, releaseRows, "被拒导入不得发版")
}

func TestService_PackImport_ReaderError(t *testing.T) {
	env := newPackEnv(t)
	_, err := env.service.PackImport(env.ctx, cwErrReader{}, 1024, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "读取上传内容失败")
}

// 声明尺寸合规但实际流超限（上传头不可信）→ 读后复检拦截。
func TestService_PackImport_StreamExceedsCap(t *testing.T) {
	env := newPackEnv(t)
	_, err := env.service.PackImport(env.ctx,
		&cwZeroReader{remaining: extensionPackMaxSize + 4096}, 1024, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "大小上限")
}

func TestService_PackImport_PackParseError(t *testing.T) {
	env := newPackEnv(t)
	pack := buildPack(t, map[string]string{"README.md": "no manifest here"})

	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest.json")
}

// 包内未给 name/displayName/vendor/kind 时按登记默认值兜底。
func TestService_PackImport_ManifestDefaults(t *testing.T) {
	env := newPackEnv(t)
	manifest := `{"extensionId": "com.example.min", "version": "1.0.0", "manifest": {}}`
	pack := buildPack(t, map[string]string{"manifest.json": manifest})

	resp, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.NoError(t, err)
	assert.Equal(t, "com.example.min", resp.Catalog.Name, "name 缺省回退 extensionId")
	assert.Equal(t, "com.example.min", resp.Catalog.DisplayName, "displayName 缺省回退 name")
	assert.Equal(t, "external", resp.Catalog.Vendor)
	assert.Equal(t, "community", resp.Catalog.Kind)
	assert.Equal(t, "active", resp.Catalog.Status)
}

func TestService_PackImport_CatalogCreateBlocked(t *testing.T) {
	env := newPackEnv(t)
	cwTrigger(t, env.db, `CREATE TRIGGER cw_block_catalog_insert BEFORE INSERT ON extension_catalogs
BEGIN SELECT RAISE(ABORT, 'cw: catalog insert blocked'); END`)

	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")})
	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: catalog insert blocked")

	var catalogRows int64
	require.NoError(t, env.db.Model(&model.ExtensionCatalog{}).Count(&catalogRows).Error)
	assert.Zero(t, catalogRows, "登记失败时不得留半行")
	var releaseRows int64
	require.NoError(t, env.db.Model(&model.ExtensionRelease{}).Count(&releaseRows).Error)
	assert.Zero(t, releaseRows, "登记失败不得继续发版")
}

func TestService_PackImport_CatalogLookupError(t *testing.T) {
	env := newPackEnv(t)
	cwDrop(t, env.db, &model.ExtensionCatalog{})

	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")})
	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such table")
}

func TestService_PackImport_ReleaseLookupError(t *testing.T) {
	env := newPackEnv(t)
	cwFailQuery(t, env.db, "cw:fail_release_version_lookup", "extension_releases", "version =")

	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")})
	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: injected query failure")
}

// 工件写入对象存储失败（底目录只读 → MkdirAll/Create 皆失败）。
func TestService_PackImport_ObjectStorePutFailure(t *testing.T) {
	env := newPackEnv(t)
	cwReadOnlyStore(t, env)

	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")})
	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "写入对象存储失败")
}

// 发版写入被拦：工件已落存储但不得留下无版本记录的导入结果。
func TestService_PackImport_PublishReleaseBlocked(t *testing.T) {
	env := newPackEnv(t)
	cwTrigger(t, env.db, `CREATE TRIGGER cw_block_release_insert BEFORE INSERT ON extension_releases
BEGIN SELECT RAISE(ABORT, 'cw: release insert blocked'); END`)

	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")})
	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: release insert blocked")

	var releaseRows int64
	require.NoError(t, env.db.Model(&model.ExtensionRelease{}).Count(&releaseRows).Error)
	assert.Zero(t, releaseRows)
}

// 存量登记（latestVersion 为空）导入时回填指针失败。
func TestService_PackImport_LatestBumpBlocked(t *testing.T) {
	env := newPackEnv(t)
	_, err := env.service.CatalogCreate(env.ctx, ExtensionCatalogCreateRequest{ExtensionID: cwExtID}, "ext_tester")
	require.NoError(t, err)
	cwTrigger(t, env.db, `CREATE TRIGGER cw_block_catalog_update BEFORE UPDATE ON extension_catalogs
BEGIN SELECT RAISE(ABORT, 'cw: catalog update blocked'); END`)

	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")})
	_, err = env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: catalog update blocked")

	item, _, err := env.svcCtx.Extensions.Catalog.Get(env.ctx, cwExtID)
	require.NoError(t, err)
	assert.Empty(t, item.LatestVersion, "回填失败不得改写指针")
}

// ---- catalogDisplayNames ----

// 归一后重复的 extensionID 与空值只取一次（避免重复查表）。
func TestCatalogDisplayNames_DedupSkipsEmpty(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "Com.Example.Demo", "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})

	items := []model.ExtensionInstallation{
		{ExtensionID: "Com.Example.Demo"},
		{ExtensionID: "com.example.demo"},
		{ExtensionID: "   "},
	}
	names := env.service.catalogDisplayNames(env.ctx, items)
	assert.Equal(t, map[string]string{"com.example.demo": "Com.Example.Demo display"}, names)

	// 全空/全重复到只剩空值时不得触达查表。
	empty := env.service.catalogDisplayNames(env.ctx, []model.ExtensionInstallation{{ExtensionID: " "}})
	assert.Empty(t, empty)
}

// 查表失败按设计静默回退 extensionID 原值（不阻塞列表）。
func TestCatalogDisplayNames_LookupErrorSwallowed(t *testing.T) {
	env := setupExtensionEnv(t)
	cwDrop(t, env.db, &model.ExtensionCatalog{})

	names := env.service.catalogDisplayNames(env.ctx, []model.ExtensionInstallation{{ExtensionID: cwExtID}})
	assert.Empty(t, names, "查表失败应返回空 map，由调用侧回退原值")
}

// ---- handler 层：写端点错误分支 ----

func TestHandler_CatalogCreate_HTTP(t *testing.T) {
	env := setupExtensionEnv(t)
	cwRegister(env)

	// 畸形 JSON 与缺必填字段都是 400。
	rec := env.do(t, http.MethodPost, "/api/v1/extensions/catalog", "{")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	rec = env.do(t, http.MethodPost, "/api/v1/extensions/catalog", `{"name":"no id"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = env.do(t, http.MethodPost, "/api/v1/extensions/catalog",
		fmt.Sprintf(`{"extensionId":%q,"name":"demo"}`, cwExtID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var created ExtensionCatalogMutateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, cwExtID, created.Item.ID)

	// 重复登记 409。
	rec = env.do(t, http.MethodPost, "/api/v1/extensions/catalog",
		fmt.Sprintf(`{"extensionId":%q}`, cwExtID))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

func TestHandler_CatalogUpdate_HTTP(t *testing.T) {
	env := setupExtensionEnv(t)
	cwRegister(env)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})

	rec := env.do(t, http.MethodPut, "/api/v1/extensions/catalog/"+cwExtID, "{")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = env.do(t, http.MethodPut, "/api/v1/extensions/catalog/"+cwExtID, `{"displayName":"Demo Pro"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var updated ExtensionCatalogMutateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Equal(t, "Demo Pro", updated.Item.DisplayName)

	rec = env.do(t, http.MethodPut, "/api/v1/extensions/catalog/com.example.absent", `{"name":"x"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestHandler_CatalogDelete_HTTP(t *testing.T) {
	env := setupExtensionEnv(t)
	cwRegister(env)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})

	rec := env.do(t, http.MethodDelete, "/api/v1/extensions/catalog/com.example.absent", "")
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	rec = env.do(t, http.MethodDelete, "/api/v1/extensions/catalog/"+cwExtID, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"deleted":true}`, rec.Body.String())

	_, _, err := env.svcCtx.Extensions.Catalog.Get(env.ctx, cwExtID)
	require.Error(t, err, "移除后登记行不得残留")
}

func TestHandler_CatalogReleasePublish_HTTP(t *testing.T) {
	env := setupExtensionEnv(t)
	cwRegister(env)
	env.seedCatalog(t, cwExtID, "1.0.0", map[string]any{"capabilities": []any{"cap.echo"}})

	rec := env.do(t, http.MethodPost, "/api/v1/extensions/catalog/"+cwExtID+"/releases", "{")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	rec = env.do(t, http.MethodPost, "/api/v1/extensions/catalog/"+cwExtID+"/releases", `{"version":"1.1.0"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "manifest 缺省应 400: "+rec.Body.String())

	rec = env.do(t, http.MethodPost, "/api/v1/extensions/catalog/"+cwExtID+"/releases",
		`{"version":"1.1.0","manifest":{"capabilities":["cap.echo"]}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var published ExtensionReleasePublishResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &published))
	assert.Equal(t, "1.1.0", published.Release.Version)
	assert.Equal(t, "stable", published.Release.ReleaseChannel)

	rec = env.do(t, http.MethodPost, "/api/v1/extensions/catalog/com.example.absent/releases",
		`{"version":"1.1.0","manifest":{}}`)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestHandler_PackImport_MissingFileField(t *testing.T) {
	env := newPackEnv(t)
	cwRegister(env)

	rec := env.do(t, http.MethodPost, "/api/v1/extensions/packs/import", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "file")
}

// 上传得到文件但包体不可解析：服务层错误须原样透出（不吞成 200）。
func TestHandler_PackImport_ServiceErrorPassthrough(t *testing.T) {
	env := newPackEnv(t)
	cwRegister(env)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", "broken-1.0.0.tgz")
	require.NoError(t, err)
	_, err = io.WriteString(fw, "not a gzip stream")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/extensions/packs/import", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "gzip")
}
