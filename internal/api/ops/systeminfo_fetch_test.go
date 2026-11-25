package ops

// Coverage sweep fill-in tests (8 remaining blocks in systeminfo.go): existing cases all
// override the update source via the package-level function variable
// (systemUpdateCheckURLFn), so **the real settings reader body** never ran.
// This file adds three groups:
//  1. systemUpdateCheckURLFn real implementation — both wings: unconfigured
//     (!ok → empty string) and configured (TrimSpace normalization);
//  2. fetchRemoteVersion outbound wings — sec.* port allowlist block, transport-layer
//     connection failure, and mid-response disconnect (body read failure); injection
//     style matches the #56/#57 outbound guard tests;
//  3. extractVersionFromManifest "manifest has no usable version field" error branch
//     (three unusable shapes: key missing / non-string / whitespace-only).

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// initSysInfoSettings initialises the settings.Current() singleton and returns the L3 store handle.
// Only platform_settings is migrated (InitLayered reads just that table).
func initSysInfoSettings(t *testing.T) *model.PlatformSettingModel {
	t.Helper()
	settings.ResetForTest()
	t.Cleanup(settings.ResetForTest)

	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/sysinfo_fetch.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PlatformSetting{}))

	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	return store
}

func putSysInfoL3(t *testing.T, store *model.PlatformSettingModel, key, raw string) {
	t.Helper()
	require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	settings.Current().Reload(context.Background(), store)
}

// Update-source reader seam with the **real** implementation: key unconfigured → ok=false →
// empty string (upstream reports "update check source not configured"); configured → the
// whitespace-trimmed value.
func TestSystemUpdateCheckURLFn_RealSettingsReader(t *testing.T) {
	settings.ResetForTest() // not initialised: Current() is nil, GetString returns ok=false
	t.Cleanup(settings.ResetForTest)
	assert.Equal(t, "", systemUpdateCheckURLFn(), "未初始化 settings 时读不到键")

	store := initSysInfoSettings(t)
	assert.Equal(t, "", systemUpdateCheckURLFn(), "L3 未配置该键时为空串")

	putSysInfoL3(t, store, settings.KeySystemUpdateCheckURL, `"  https://example.com/rel.json  "`)
	assert.Equal(t, "https://example.com/rel.json", systemUpdateCheckURLFn(), "值须去首尾空白")
}

// When sec.allowPorts does not include the target port, the outbound guard rejects statically
// before dialing (ahead of HTTPClient's dial hook, so no real request goes out).
func TestFetchRemoteVersion_BlockedByOutboundGuard(t *testing.T) {
	store := initSysInfoSettings(t)
	putSysInfoL3(t, store, settings.KeySecAllowPorts, `"443"`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("被守卫拦截的目标不应收到请求")
		_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	t.Cleanup(srv.Close)

	version, err := fetchRemoteVersion(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Empty(t, version)
	assert.Contains(t, err.Error(), "不在允许清单内")
}

// Target address has no listener: CheckURL passes (guards all off), transport layer fails.
func TestFetchRemoteVersion_TransportFailure(t *testing.T) {
	_ = initSysInfoSettings(t) // guards all off by default → passes through to the transport layer

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	deadAddr := ln.Addr().String()
	require.NoError(t, ln.Close()) // the port now has no listener

	version, err := fetchRemoteVersion(context.Background(), "http://"+deadAddr+"/rel.json")
	require.Error(t, err)
	assert.Empty(t, version)
}

// Response declares a Content-Length larger than the bytes actually sent, then drops the
// connection: status 200 but the body read fails.
func TestFetchRemoteVersion_BodyReadFailure(t *testing.T) {
	_ = initSysInfoSettings(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, buf, hijackErr := w.(http.Hijacker).Hijack()
		if hijackErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 4096\r\n\r\n")
		_, _ = buf.WriteString(`{"version":"1.0.0"}`)
		_ = buf.Flush()
	}))
	t.Cleanup(srv.Close)

	version, err := fetchRemoteVersion(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Empty(t, version)
	assert.Contains(t, err.Error(), "unexpected EOF")
}

// Errors when the manifest is valid JSON but carries no usable version field (missing /
// non-string / whitespace-only); trims and returns the value of the first non-empty string key.
func TestExtractVersionFromManifest_UnusableVersionKeys(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":"release","id":7}`,
		`{"version":123}`,            // non-string
		`{"tagName":"   "}`,          // whitespace only
		`{"version":"","tagName":9}`, // empty string + non-string
	} {
		version, err := extractVersionFromManifest([]byte(body))
		require.Error(t, err, "body=%s", body)
		assert.Empty(t, version)
		assert.Contains(t, err.Error(), "缺少版本字段")
	}

	version, err := extractVersionFromManifest([]byte(`{"version":"  " , "latestVersion":" v1.4.0 "}`))
	require.NoError(t, err)
	assert.Equal(t, "v1.4.0", version, "跳过不可用键取首个可用值并去空白")
}
