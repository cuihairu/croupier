package ops

// 第三方服务健康探针端点测试（OPEN-ISSUES #57）：渠道白名单（未知 404）、
// 未配置目标 configured=false、URL 渠道对 httptest 目标探活成功、SMTP
// 未配置降级、update 渠道经 systemUpdateCheckURLFn 读取 L3。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedStore tests-only：settings.Current() 的底层 store 句柄（写 L3 + Reload 用）。
var seedStore *model.PlatformSettingModel

func setupProbeRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	settings.ResetForTest()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/probe.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	seedStore = model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, seedStore)
	t.Cleanup(settings.ResetForTest)

	r := gin.New()
	h := NewHandler(NewService(nil))
	r.POST("/probes/:channel", h.ThirdPartyProbe)
	return r
}

func seedProbeSetting(t *testing.T, key, val string) {
	t.Helper()
	require.NoError(t, seedStore.Set(context.Background(), key, json.RawMessage(`"`+val+`"`), "tester"))
	settings.Current().Reload(context.Background(), seedStore)
}

func postProbe(r *gin.Engine, channel string) (int, string) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/probes/"+channel, nil)
	r.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestThirdPartyProbe(t *testing.T) {
	r := setupProbeRouter(t)

	// 未知渠道 404
	code, body := postProbe(r, "ldap")
	require.Equal(t, http.StatusNotFound, code)
	assert.Contains(t, body, "未知探针渠道")

	// 未配置：configured=false（200）
	code, body = postProbe(r, "dingtalk")
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `"configured":false`)

	// SMTP 未配置同形
	_, body = postProbe(r, "smtp")
	assert.Contains(t, body, `"configured":false`)

	// webhook 渠道：L3 配置 httptest 目标 → 探活成功
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	seedProbeSetting(t, settings.KeyNotifyWebhookURL, srv.URL)
	code, body = postProbe(r, "webhook")
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `"configured":true`)
	assert.Contains(t, body, `"ok":true`)
	assert.Contains(t, body, `"status":200`)

	// update 渠道：经 systemUpdateCheckURLFn 读 L3
	seedProbeSetting(t, settings.KeySystemUpdateCheckURL, srv.URL)
	_, body = postProbe(r, "update")
	assert.Contains(t, body, `"ok":true`)

	// 5xx 目标 = 可达但不健康
	srv503 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv503.Close()
	seedProbeSetting(t, settings.KeyNotifyWebhookURL, srv503.URL)
	_, body = postProbe(r, "webhook")
	assert.Contains(t, body, `"ok":false`)
	assert.Contains(t, body, `"status":503`)
}
