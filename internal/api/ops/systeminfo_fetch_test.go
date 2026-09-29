package ops

// 覆盖率巡检补测（systeminfo.go 残余 8 块）：既有用例一律以包级函数变量
// （systemUpdateCheckURLFn）覆盖更新源，因此**真实的 settings 读取体**从未
// 执行；此处补三条：
//  1. systemUpdateCheckURLFn 真实实现——未配置（!ok → 空串）与已配置
//     （TrimSpace 归一）两翼；
//  2. fetchRemoteVersion 的出站三翼——sec.* 端口白名单拦截、传输层连接
//     失败、响应体中途断开（读失败）；注入口径与 #56/#57 出站守卫一致；
//  3. extractVersionFromManifest 的「清单无可用版本字段」错误分支
//     （键缺失 / 非字符串 / 纯空白三类不可用形态）。

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

// initSysInfoSettings 初始化 settings.Current() 单例并返回 L3 store 句柄。
// 仅迁移 platform_settings（InitLayered 只读该表）。
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

// 更新源读取缝隙的**真实**实现：未配置键 → ok=false → 空串（上层据此回
// 「未配置更新检查源」注记）；已配置 → 去空白归一后的值。
func TestSystemUpdateCheckURLFn_RealSettingsReader(t *testing.T) {
	settings.ResetForTest() // 未 Init：Current() 为 nil，GetString 返回 ok=false
	t.Cleanup(settings.ResetForTest)
	assert.Equal(t, "", systemUpdateCheckURLFn(), "未初始化 settings 时读不到键")

	store := initSysInfoSettings(t)
	assert.Equal(t, "", systemUpdateCheckURLFn(), "L3 未配置该键时为空串")

	putSysInfoL3(t, store, settings.KeySystemUpdateCheckURL, `"  https://example.com/rel.json  "`)
	assert.Equal(t, "https://example.com/rel.json", systemUpdateCheckURLFn(), "值须去首尾空白")
}

// sec.allowPorts 白名单不含目标端口时，出站守卫在拨号前静态拒绝
// （HTTPClient 的拨号钩子之前，故不会有真实请求发出）。
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

// 目标地址无监听：CheckURL 放行（守卫全关），传输层连接失败。
func TestFetchRemoteVersion_TransportFailure(t *testing.T) {
	_ = initSysInfoSettings(t) // 守卫默认全关 → 放行到传输层

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	deadAddr := ln.Addr().String()
	require.NoError(t, ln.Close()) // 端口随即无人监听

	version, err := fetchRemoteVersion(context.Background(), "http://"+deadAddr+"/rel.json")
	require.Error(t, err)
	assert.Empty(t, version)
}

// 响应声明 Content-Length 大于实际下发字节后直接断连：状态 200 但读体失败。
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

// 清单合法 JSON 但无任何可用版本字段（缺失 / 非字符串 / 纯空白）时报错；
// 命中首个非空字符串键时去空白返回。
func TestExtractVersionFromManifest_UnusableVersionKeys(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":"release","id":7}`,
		`{"version":123}`,            // 非字符串
		`{"tagName":"   "}`,          // 纯空白
		`{"version":"","tagName":9}`, // 空串 + 非字符串
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
