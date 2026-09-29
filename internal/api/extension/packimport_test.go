package extension

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildPack 把多文件内容打成 .tgz 字节流（键为 tar 条目名，含目录前缀语义）。
func buildPack(t *testing.T, files map[string]string) []byte {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, name := range names {
		body := files[name]
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

const packManifestTemplate = `{
  "extensionId": %q,
  "name": "demo",
  "displayName": "Demo Extension",
  "vendor": "example",
  "kind": "integration",
  "summary": "demo pack",
  "version": %q,
  "releaseChannel": %q,
  "minCoreVersion": "0.0.1",
  "changelog": "first release",
  "manifest": {"capabilities": ["cap.echo"], "pages": []}
}`

func newPackEnv(t *testing.T) *extensionTestEnv {
	t.Helper()
	env := setupExtensionEnv(t)
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	env.svcCtx.ObjectStore = store
	return env
}

func TestParseExtensionPack(t *testing.T) {
	valid := fmt.Sprintf(packManifestTemplate, "com.example.demo", "1.0.0", "stable")

	// 包根（./ 前缀）与单层顶层目录均容忍，多层嵌套取最浅。
	for name, files := range map[string]map[string]string{
		"root":         {"./manifest.json": valid, "schemas/a.json": "{}"},
		"top-dir":      {"pack/manifest.json": valid, "pack/schemas/a.json": "{}"},
		"shallow-wins": {"deep/a/manifest.json": valid, "manifest.json": valid},
	} {
		t.Run(name, func(t *testing.T) {
			m, err := parseExtensionPack(buildPack(t, files))
			require.NoError(t, err)
			assert.Equal(t, "com.example.demo", m.ExtensionID)
			assert.Equal(t, "1.0.0", m.Version)
			assert.JSONEq(t, `{"capabilities": ["cap.echo"], "pages": []}`, string(m.Manifest))
		})
	}

	// 非 gzip / 非 tar / 缺 manifest / 坏 JSON → 400 语义。
	badCases := []struct {
		name string
		data []byte
		want string
	}{
		{"not gzip", []byte("plain text"), "gzip"},
		{"gzip but not tar", func() []byte {
			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			_, _ = gw.Write([]byte("junk"))
			_ = gw.Close()
			return buf.Bytes()
		}(), "tar"},
		{"missing manifest", buildPack(t, map[string]string{"README.md": "hi"}), "manifest.json"},
		{"bad json", buildPack(t, map[string]string{"manifest.json": "{nope"}), "JSON"},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseExtensionPack(tc.data)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			var bad *errorx.CodeError
			assert.ErrorAs(t, err, &bad)
		})
	}
}

func TestService_PackImport_CreatesCatalogAndRelease(t *testing.T) {
	env := newPackEnv(t)
	pack := buildPack(t, map[string]string{"./manifest.json": fmt.Sprintf(packManifestTemplate, "com.example.demo", "1.0.0", "stable")})

	resp, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.NoError(t, err)
	assert.True(t, resp.CatalogCreated)
	assert.Equal(t, "com.example.demo", resp.Catalog.ID)
	assert.Equal(t, "Demo Extension", resp.Catalog.DisplayName)
	assert.Equal(t, "1.0.0", resp.Catalog.LatestVersion)
	assert.Equal(t, "1.0.0", resp.Release.Version)
	assert.Equal(t, "stable", resp.Release.ReleaseChannel)
	assert.Equal(t, "first release", resp.Release.Changelog)
	assert.Equal(t, "extension-packs/com.example.demo/1.0.0.tgz", resp.PackageRef)
	assert.True(t, len(resp.Checksum) == len("sha256:")+64, "checksum 应为 sha256:<64hex>，got %s", resp.Checksum)
	assert.Equal(t, int64(len(pack)), resp.Size)

	// 工件真实落库对象存储。
	objs, err := env.svcCtx.ObjectStore.List(env.ctx, "extension-packs/", "", "", 10)
	require.NoError(t, err)
	require.Len(t, objs.Objects, 1)
	assert.Equal(t, "extension-packs/com.example.demo/1.0.0.tgz", objs.Objects[0].Key)

	// manifest 落 release 行（经读路径校验回环）。
	item, releases, err := env.svcCtx.Extensions.Catalog.Get(env.ctx, "com.example.demo")
	require.NoError(t, err)
	require.Len(t, releases, 1)
	var manifest map[string]any
	require.NoError(t, json.Unmarshal([]byte(releases[0].ManifestJSON), &manifest))
	assert.Equal(t, []any{"cap.echo"}, manifest["capabilities"])
	assert.Equal(t, "com.example.demo", item.ExtensionID)
}

func TestService_PackImport_ReuseCatalogAndBumpLatest(t *testing.T) {
	env := newPackEnv(t)
	pack100 := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, "com.example.demo", "1.0.0", "stable")})
	pack120 := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, "com.example.demo", "1.2.0", "beta")})

	first, err := env.service.PackImport(env.ctx, bytes.NewReader(pack100), int64(len(pack100)), "ext_tester")
	require.NoError(t, err)
	assert.True(t, first.CatalogCreated)

	second, err := env.service.PackImport(env.ctx, bytes.NewReader(pack120), int64(len(pack120)), "ext_tester")
	require.NoError(t, err)
	assert.False(t, second.CatalogCreated, "已登记扩展再导入只补版本，不重建 catalog 行")
	assert.Equal(t, "1.2.0", second.Catalog.LatestVersion, "更高 semver 回填 latestVersion")
	assert.Equal(t, "beta", second.Release.ReleaseChannel)
}

func TestService_PackImport_DuplicateVersionConflict(t *testing.T) {
	env := newPackEnv(t)
	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, "com.example.demo", "1.0.0", "stable")})
	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.NoError(t, err)

	_, err = env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release version already exists")
}

func TestService_PackImport_Validation(t *testing.T) {
	env := newPackEnv(t)
	cases := []struct {
		name     string
		manifest string
		want     string
	}{
		{"missing extensionId", `{"version": "1.0.0", "manifest": {}}`, "extensionId"},
		{"bad extensionId", fmt.Sprintf(packManifestTemplate, "Com.Example", "1.0.0", "stable"), "extensionId"},
		{"bad semver", fmt.Sprintf(packManifestTemplate, "com.example.demo", "not-a-version", "stable"), "semver"},
		{"bad channel", fmt.Sprintf(packManifestTemplate, "com.example.demo", "1.0.0", "rc"), "releaseChannel"},
		{"manifest not object", `{"extensionId": "com.example.demo", "version": "1.0.0", "manifest": [1]}`, "JSON 对象"},
		{"manifest missing", `{"extensionId": "com.example.demo", "version": "1.0.0"}`, "JSON 对象"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pack := buildPack(t, map[string]string{"manifest.json": tc.manifest})
			_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestService_PackImport_NoObjectStore(t *testing.T) {
	env := setupExtensionEnv(t) // ObjectStore 恒 nil
	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, "com.example.demo", "1.0.0", "stable")})
	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "对象存储未配置")
}

func TestService_PackImport_Oversize(t *testing.T) {
	env := newPackEnv(t)
	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, "com.example.demo", "1.0.0", "stable")})
	_, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), extensionPackMaxSize+1, "ext_tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "大小上限")
}

func TestHandler_PackImport_Multipart(t *testing.T) {
	env := newPackEnv(t)
	env.router.POST("/api/v1/extensions/packs/import", env.handler.PackImport)

	pack := buildPack(t, map[string]string{"manifest.json": fmt.Sprintf(packManifestTemplate, "com.example.demo", "1.0.0", "stable")})
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "demo-1.0.0.tgz")
	require.NoError(t, err)
	_, err = io.WriteString(fw, string(pack))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/extensions/packs/import", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp ExtensionPackImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "com.example.demo", resp.Catalog.ID)
	assert.True(t, resp.CatalogCreated)
	assert.Equal(t, "1.0.0", resp.Release.Version)
}
