package extension

// Error-arm supplemental tests for the catalog write path (#46 batch 3:
// register/update/delete/publish) and pack import (batch 6): batches 3/6 only
// covered the success mainline, and the error branches of the five write
// endpoints in handler.go plus the storage-failure arms on the service side
// were entirely uncovered (package coverage stalled at 94.7%, handler write
// segment at 0%).
//
// Injection conventions follow the repo's existing batches:
//   - Read arm "missing table": after DropTable, gorm errors immediately with
//     no side effects, hitting the "non-NotFound passes through as-is" arm
//     (if it were misjudged as NotFound it would enter the duplicate-register
//     / create branches, which this cannot detect);
//   - Write arm "trigger blocks write": BEFORE {INSERT,UPDATE,DELETE} TRIGGER
//     + RAISE(ABORT), covering the class where all validation passes and it
//     only blows up when the SQL actually executes;
//   - "Write succeeds -> re-read fails": a BEFORE UPDATE trigger soft-deletes
//     the row -- UpdateFields still reports 1 row affected (the repository
//     checks RowsAffected for emptiness), and the immediate re-read lands in
//     NotFound;
//   - Permission arm: ctx carries no username, RequireAnyPermission stops at
//     LoadCurrentAdmin.
//
// Unreachable register: PackImport line 513 (json.Marshal(manifestObj)
// failure) -- the only source of the input is the map[string]any obtained by
// json.Unmarshal from the in-pack manifest.json, whose values can only be
// string/float64/bool/nil/map/slice, so Marshal has no failure mode
// (chan/func/NaN cannot be produced from JSON); per house rules we neither
// fabricate a case nor delete the defensive branch.

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

// cwDrop read-arm injection: missing table (table name taken from the model's
// TableName, not hand-written).
func cwDrop(t *testing.T, db *gorm.DB, m any) {
	t.Helper()
	require.NoError(t, db.Migrator().DropTable(m))
}

// cwTrigger write-arm injection: a blocking trigger that only fires when the
// SQL actually executes.
func cwTrigger(t *testing.T, db *gorm.DB, sql string) {
	t.Helper()
	require.NoError(t, db.Exec(sql).Error)
}

// cwRegister registers the five write-path endpoints (paths consistent with
// the registration order in internal/handler/routes.go).
func cwRegister(env *extensionTestEnv) {
	env.router.POST("/api/v1/extensions/catalog", env.handler.CatalogCreate)
	env.router.PUT("/api/v1/extensions/catalog/:id", env.handler.CatalogUpdate)
	env.router.DELETE("/api/v1/extensions/catalog/:id", env.handler.CatalogDelete)
	env.router.POST("/api/v1/extensions/catalog/:id/releases", env.handler.CatalogReleasePublish)
	env.router.POST("/api/v1/extensions/packs/import", env.handler.PackImport)
}

// cwFailQuery injects a storage failure targeting one specific query shape:
// the catalog Get also reads releases internally, so a missing-table injection
// cannot reach the "all tables exist, only this version lookup fails" shape;
// therefore an error is injected on the gorm query callback for SQL matching
// the table name + substring.
func cwFailQuery(t *testing.T, db *gorm.DB, name, table, substr string) {
	t.Helper()
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == table && strings.Contains(tx.Statement.SQL.String(), substr) {
			tx.AddError(errors.New("cw: injected query failure"))
		}
	}))
}

// cwReadOnlyStore makes the object storage base directory read-only (Put's
// MkdirAll/Create will then fail). Permissions are restored before the TempDir
// cleanup (cleanup is last-in-first-out; this registration happens after
// TempDir).
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
	// Length cap (129 > 128).
	err := validateCatalogExtensionID(strings.Repeat("a", 129))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "128")

	// The leading character forbids . _ - (route parameter ambiguity guard).
	for _, bad := range []string{".lead", "_lead", "-lead"} {
		err := validateCatalogExtensionID(bad)
		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), "start with", bad)
	}

	// Legal shapes inside the boundary must not be wrongly rejected.
	assert.NoError(t, validateCatalogExtensionID(strings.Repeat("a", 128)))
	assert.NoError(t, validateCatalogExtensionID("a.lead-_1"))
}

// ---- CatalogCreate ----

// When the pre-registration existence probe returns a non-NotFound error it is
// passed through as-is (it must not be treated as "creatable").
func TestService_CatalogCreate_LookupErrorPassthrough(t *testing.T) {
	env := setupExtensionEnv(t)
	cwDrop(t, env.db, &model.ExtensionCatalog{})

	_, err := env.service.CatalogCreate(env.ctx, ExtensionCatalogCreateRequest{ExtensionID: cwExtID}, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such table", "缺表属非 NotFound，应原样透传而非当未登记处理")
}

// Write-blocking arm where all validation passes and the INSERT is what fails.
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

	// ctx carries no username: the permission guard stops at the admin loading
	// stage, so the write path must not land anything in the database.
	_, err := env.service.CatalogUpdate(context.Background(), cwExtID, ExtensionCatalogUpdateRequest{Name: "renamed"})
	require.Error(t, err)

	item, _, err := env.svcCtx.Extensions.Catalog.Get(env.ctx, cwExtID)
	require.NoError(t, err)
	assert.Equal(t, cwExtID, item.Name, "被拒请求不得改写登记行")
}

// The three non-empty override branches for name/vendor/kind (the remaining
// fields are written unconditionally and are irrelevant to this case).
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

// "Write succeeds -> re-read fails" arm: a BEFORE UPDATE trigger soft-deletes
// the row, UpdateFields still counts 1 affected row (the repository checks
// RowsAffected for emptiness), and the following re-read lands in NotFound.
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

// Release cleanup (RemoveReleases) blocked: the catalog row must still be
// there; no half-deleted state is allowed.
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

// Read arm of the active installation probe.
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
	// Only the "dedupe lookup by extension_id+version" fails: the preceding
	// catalog Get (which internally reads the releases list) is unaffected, so
	// this cannot be injected by a missing table.
	cwFailQuery(t, env.db, "cw:fail_release_version_lookup", "extension_releases", "version =")

	_, err := env.service.CatalogReleasePublish(env.ctx, cwExtID, ExtensionReleasePublishRequest{
		Version:  "1.1.0",
		Manifest: map[string]any{"capabilities": []any{"cap.echo"}},
	}, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cw: injected query failure")
}

// The publish-release write (PublishRelease) is blocked.
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

// The version was published but the latestVersion back-fill write fails: the
// API must report an error and must not silently leave the intermediate
// state of "released, pointer not back-filled".
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

// Manifest-unserializable arm: the service layer is an exported method, so an
// in-process caller can pass a value from a non-JSON source.
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

// When an unparsable pre-existing latestVersion is treated as "unknown ->
// back-fill" (not back-filling the pointer would lose the semantics).
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

// A second manifest at the same depth is skipped (the first one found wins;
// there is no overwrite).
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

// Truncated mid-archive: the entry header declares a size but the data is
// short, so reading the manifest fails.
func TestParseExtensionPack_TruncatedManifestRead(t *testing.T) {
	body := fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o644, Size: int64(len(body))}))
	_, err := tw.Write([]byte(body))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	// Keep only a 512-byte entry header + 100-byte body (the declared size is
	// larger), so the entry data is short.
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

// cwZeroReader produces a zero stream of a given length (avoids actually
// building a 64MiB buffer).
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

// Declared size is compliant but the actual stream exceeds the cap (the upload
// header is untrusted) -> intercepted by re-checking after the read.
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

// When the pack gives no name/displayName/vendor/kind, the registration
// defaults are used as the fallback.
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

// Writing the artifact to object storage fails (read-only base directory ->
// both MkdirAll/Create fail).
func TestService_PackImport_ObjectStorePutFailure(t *testing.T) {
	env := newPackEnv(t)
	cwReadOnlyStore(t, env)

	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, cwExtID, "1.0.0", "stable")})
	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "写入对象存储失败")
}

// The publish-release write is blocked: the artifact has already landed in
// storage but the import must not leave a result without a version record.
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

// Back-filling the pointer fails on import for a pre-existing registration
// (latestVersion empty).
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

// extensionIDs that are duplicates after normalization and empty values are
// fetched only once (avoids repeated lookups).
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

	// When everything is empty/duplicate down to a single empty value, the
	// lookup must not be touched.
	empty := env.service.catalogDisplayNames(env.ctx, []model.ExtensionInstallation{{ExtensionID: " "}})
	assert.Empty(t, empty)
}

// On lookup failure the raw extensionID is silently restored by design (the
// list is not blocked).
func TestCatalogDisplayNames_LookupErrorSwallowed(t *testing.T) {
	env := setupExtensionEnv(t)
	cwDrop(t, env.db, &model.ExtensionCatalog{})

	names := env.service.catalogDisplayNames(env.ctx, []model.ExtensionInstallation{{ExtensionID: cwExtID}})
	assert.Empty(t, names, "查表失败应返回空 map，由调用侧回退原值")
}

// ---- handler layer: write endpoint error branches ----

func TestHandler_CatalogCreate_HTTP(t *testing.T) {
	env := setupExtensionEnv(t)
	cwRegister(env)

	// Both malformed JSON and a missing required field are 400.
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

	// Duplicate registration is 409.
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

// A file comes through but the pack body is unparseable: the service-layer
// error must pass through untouched (not be swallowed into a 200).
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
