package alert

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// inboundTestSecret 测试用共享密钥（环境变量引用路径的值）。
const inboundTestSecret = "test-inbound-secret"

func setupInboundServer(t *testing.T, sources map[string]config.AlertInboundSourceConfig) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	svcCtx.Config.AlertInbound.Sources = sources
	h := NewHandler(NewService(svcCtx))
	e := gin.New()
	e.POST("/api/v1/alerts/inbound/:source", h.Inbound)
	return e, db
}

// signedInboundRequest 构造带 HMAC 签名与时间戳的入站请求。
func signedInboundRequest(t *testing.T, source string, body []byte, secret string, tsUnix int64) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/inbound/"+source, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		req.Header.Set("X-Croupier-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	if tsUnix != 0 {
		req.Header.Set("X-Croupier-Timestamp", strconv.FormatInt(tsUnix, 10))
	}
	return req
}

func doInbound(t *testing.T, e *gin.Engine, req *http.Request) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func TestInboundUnknownSourceRejected(t *testing.T) {
	e, _ := setupInboundServer(t, nil)
	code, _ := doInbound(t, e, signedInboundRequest(t, "unknown", []byte(`{}`), inboundTestSecret, time.Now().Unix()))
	assert.Equal(t, http.StatusNotFound, code)
}

func TestInboundSourceNotConfigured(t *testing.T) {
	// 来源在闭集但未配置 secretEnv → 403（非开放端点）。
	e, _ := setupInboundServer(t, map[string]config.AlertInboundSourceConfig{})
	code, _ := doInbound(t, e, signedInboundRequest(t, "generic", []byte(`{}`), inboundTestSecret, time.Now().Unix()))
	assert.Equal(t, http.StatusForbidden, code)
}

func TestInboundSecretEnvMissing(t *testing.T) {
	// secretEnv 指向的环境变量为空 → 403（降级拒绝，不开放）。
	t.Setenv("INBOUND_TEST_EMPTY_SECRET", "")
	e, _ := setupInboundServer(t, map[string]config.AlertInboundSourceConfig{
		"generic": {SecretEnv: "INBOUND_TEST_EMPTY_SECRET"},
	})
	code, _ := doInbound(t, e, signedInboundRequest(t, "generic", []byte(`{}`), inboundTestSecret, time.Now().Unix()))
	assert.Equal(t, http.StatusForbidden, code)
}

func TestInboundSignatureRejected(t *testing.T) {
	t.Setenv("INBOUND_TEST_SECRET", inboundTestSecret)
	e, _ := setupInboundServer(t, map[string]config.AlertInboundSourceConfig{
		"generic": {SecretEnv: "INBOUND_TEST_SECRET"},
	})
	body := []byte(`{"eventId":"sig-1","severity":"warning"}`)
	// 错误密钥签名 → 400 signature_invalid。
	code, respBody := doInbound(t, e, signedInboundRequest(t, "generic", body, "wrong-secret", time.Now().Unix()))
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "signature_invalid", respBody["error"])
	// 缺签名头 → 400。
	req := signedInboundRequest(t, "generic", body, "", time.Now().Unix())
	code, _ = doInbound(t, e, req)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestInboundTimestampReplayRejected(t *testing.T) {
	t.Setenv("INBOUND_TEST_SECRET", inboundTestSecret)
	e, _ := setupInboundServer(t, map[string]config.AlertInboundSourceConfig{
		"generic": {SecretEnv: "INBOUND_TEST_SECRET"},
	})
	body := []byte(`{"eventId":"ts-1","severity":"info"}`)
	// 缺时间戳头 → 400。
	code, respBody := doInbound(t, e, signedInboundRequest(t, "generic", body, inboundTestSecret, 0))
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "timestamp_invalid", respBody["error"])
	// 超窗口（10 分钟前）→ 400（防重放）。
	stale := time.Now().Add(-10 * time.Minute).Unix()
	code, respBody = doInbound(t, e, signedInboundRequest(t, "generic", body, inboundTestSecret, stale))
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "timestamp_invalid", respBody["error"])
	// 非法时间戳 → 400。
	req := signedInboundRequest(t, "generic", body, inboundTestSecret, time.Now().Unix())
	req.Header.Set("X-Croupier-Timestamp", "not-a-number")
	code, _ = doInbound(t, e, req)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestInboundGenericAcceptedAndMapped(t *testing.T) {
	t.Setenv("INBOUND_TEST_SECRET", inboundTestSecret)
	e, db := setupInboundServer(t, map[string]config.AlertInboundSourceConfig{
		"generic": {SecretEnv: "INBOUND_TEST_SECRET"},
	})
	body := []byte(`{
		"kind": "alert.inbound",
		"severity": "critical",
		"eventId": "ext-123",
		"dedupKey": "generic:ext-123",
		"title": "Disk full",
		"body": "usage 95%",
		"occurredAtUnixMs": 1760000000000,
		"scope": {"gameId": "g1", "env": "prod"},
		"labels": {"source": "zabbix"}
	}`)
	code, respBody := doInbound(t, e, signedInboundRequest(t, "generic", body, inboundTestSecret, time.Now().Unix()))
	require.Equal(t, http.StatusCreated, code)
	created, ok := respBody["created"].(bool)
	require.True(t, ok)
	assert.True(t, created)

	alertMap, ok := respBody["alert"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ext-123", alertMap["id"])
	assert.Equal(t, "alert.inbound", alertMap["type"])
	assert.Equal(t, "critical", alertMap["level"])
	assert.Equal(t, "generic", alertMap["source"])
	assert.Equal(t, "firing", alertMap["status"])
	assert.Contains(t, alertMap["message"], "Disk full")
	assert.Contains(t, alertMap["message"], "usage 95%")

	// 落库断言（告警页同源存储）。
	var row model.Alert
	require.NoError(t, db.Where("alert_id = ?", "ext-123").First(&row).Error)
	assert.Equal(t, "generic", row.Source)
	assert.NotNil(t, row.Details)
}

func TestInboundGenericIdempotent(t *testing.T) {
	t.Setenv("INBOUND_TEST_SECRET", inboundTestSecret)
	e, db := setupInboundServer(t, map[string]config.AlertInboundSourceConfig{
		"generic": {SecretEnv: "INBOUND_TEST_SECRET"},
	})
	body := []byte(`{"kind":"alert.inbound","severity":"warning","eventId":"dup-1","title":"t"}`)
	code1, resp1 := doInbound(t, e, signedInboundRequest(t, "generic", body, inboundTestSecret, time.Now().Unix()))
	require.Equal(t, http.StatusCreated, code1)
	// 同 eventId 重推 → 200 created=false（幂等折叠，不重复落库）。
	code2, resp2 := doInbound(t, e, signedInboundRequest(t, "generic", body, inboundTestSecret, time.Now().Unix()))
	assert.Equal(t, http.StatusOK, code2)
	created, _ := resp2["created"].(bool)
	assert.False(t, created)
	assert.Equal(t, resp1["alert"].(map[string]any)["id"], resp2["alert"].(map[string]any)["id"])

	var count int64
	require.NoError(t, db.Model(&model.Alert{}).Where("alert_id = ?", "dup-1").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestInboundGenericPayloadInvalid(t *testing.T) {
	t.Setenv("INBOUND_TEST_SECRET", inboundTestSecret)
	e, _ := setupInboundServer(t, map[string]config.AlertInboundSourceConfig{
		"generic": {SecretEnv: "INBOUND_TEST_SECRET"},
	})
	// 缺 eventId → 400。
	code, _ := doInbound(t, e, signedInboundRequest(t, "generic", []byte(`{"severity":"info"}`), inboundTestSecret, time.Now().Unix()))
	assert.Equal(t, http.StatusBadRequest, code)
	// 非法 JSON → 400。
	code, _ = doInbound(t, e, signedInboundRequest(t, "generic", []byte(`not-json`), inboundTestSecret, time.Now().Unix()))
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestInboundAlertmanagerAcceptedAndMapped(t *testing.T) {
	t.Setenv("INBOUND_TEST_SECRET", inboundTestSecret)
	e, _ := setupInboundServer(t, map[string]config.AlertInboundSourceConfig{
		"alertmanager": {SecretEnv: "INBOUND_TEST_SECRET"}, // 未配 labelMapping → 内置默认映射
	})
	body := []byte(`{
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "HighCPU", "severity": "critical", "instance": "node-1"},
			"annotations": {"summary": "CPU over 90%", "description": "sustained load"},
			"generatorURL": "http://prometheus/graph"
		}]
	}`)
	code, respBody := doInbound(t, e, signedInboundRequest(t, "alertmanager", body, inboundTestSecret, time.Now().Unix()))
	require.Equal(t, http.StatusCreated, code)

	alertMap := respBody["alert"].(map[string]any)
	assert.Equal(t, "critical", alertMap["level"])
	// 默认映射：instance→type（ops 页 Service 列显示实例名）。
	assert.Equal(t, "node-1", alertMap["type"])
	assert.Equal(t, "alertmanager", alertMap["source"])
	assert.Equal(t, "firing", alertMap["status"])
	// annotations.summary 兜底 message。
	assert.Contains(t, alertMap["message"], "CPU over 90%")

	// 幂等键确定性：同 payload 重推 → created=false。
	code2, respBody2 := doInbound(t, e, signedInboundRequest(t, "alertmanager", body, inboundTestSecret, time.Now().Unix()))
	assert.Equal(t, http.StatusOK, code2)
	created2, _ := respBody2["created"].(bool)
	assert.False(t, created2)
	assert.Equal(t, alertMap["id"], respBody2["alert"].(map[string]any)["id"])
}

func TestInboundAlertmanagerCustomLabelMapping(t *testing.T) {
	t.Setenv("INBOUND_TEST_SECRET", inboundTestSecret)
	e, _ := setupInboundServer(t, map[string]config.AlertInboundSourceConfig{
		"alertmanager": {
			SecretEnv:    "INBOUND_TEST_SECRET",
			LabelMapping: map[string]string{"priority": "level", "component": "type"},
		},
	})
	body := []byte(`{
		"alerts": [{
			"status": "resolved",
			"labels": {"priority": "warning", "component": "gameserver"},
			"annotations": {"summary": "recovered"}
		}]
	}`)
	code, respBody := doInbound(t, e, signedInboundRequest(t, "alertmanager", body, inboundTestSecret, time.Now().Unix()))
	require.Equal(t, http.StatusCreated, code)
	alertMap := respBody["alert"].(map[string]any)
	// 配置驱动映射覆盖内置默认。
	assert.Equal(t, "warning", alertMap["level"])
	assert.Equal(t, "gameserver", alertMap["type"])
	assert.Equal(t, "resolved", alertMap["status"])
}

func TestUpsertByAlertID(t *testing.T) {
	db := setupAlertTestDB(t)
	m := model.NewAlertModel(db)

	row, created, err := m.UpsertByAlertID(context.Background(), &model.Alert{
		AlertID: "up-1", Type: "alert.inbound", Level: "critical",
		Message: "m", Source: "generic", Status: "firing",
	})
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotZero(t, row.ID)

	// 同 id 再写 → 返回已存在行（dedup）。
	row2, created2, err := m.UpsertByAlertID(context.Background(), &model.Alert{
		AlertID: "up-1", Type: "alert.inbound", Level: "info", Message: "second",
	})
	require.NoError(t, err)
	assert.False(t, created2)
	assert.Equal(t, row.ID, row2.ID)
	assert.Equal(t, "critical", row2.Level) // 保留首条内容

	// 空 alert_id → 报错。
	_, _, err = m.UpsertByAlertID(context.Background(), &model.Alert{})
	assert.Error(t, err)
}

func TestInboundAlertVisibleOnOpsList(t *testing.T) {
	// 告警页落库回归：入站告警（source=alertmanager/generic）不在 ops 列表
	// 排除面（ExcludeSources=contract），落库即页面可见。
	db := setupAlertTestDB(t)
	m := model.NewAlertModel(db)
	for _, id := range []string{"ops-vis-am", "ops-vis-gen"} {
		_, _, err := m.UpsertByAlertID(context.Background(), &model.Alert{
			AlertID: id, Type: "alert.inbound", Level: "warning",
			Message: "m", Source: "generic", Status: "firing",
		})
		require.NoError(t, err)
	}
	// contract 来源是 ops 列表的既有排除面（函数域业务告警不混入）。
	_, _, err := m.UpsertByAlertID(context.Background(), &model.Alert{
		AlertID: "ops-vis-contract", Type: "contract", Level: "warning",
		Message: "m", Source: "contract", Status: "firing",
	})
	require.NoError(t, err)
	items, total, err := m.List(context.Background(), model.ListAlertsOptions{ExcludeSources: []string{"contract"}})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))
	ids := map[string]bool{}
	for _, it := range items {
		ids[it.AlertID] = true
	}
	assert.True(t, ids["ops-vis-am"])
	assert.True(t, ids["ops-vis-gen"])
	assert.False(t, ids["ops-vis-contract"])
}
