package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func newAlertRulesEnv(t *testing.T) (*gin.Engine, *Service, *gorm.DB) {
	t.Helper()
	name := fmt.Sprintf("alert_rules_%s", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	s := NewService(&svc.ServiceContext{
		DB:             db,
		AlertModel:     model.NewAlertModel(db),
		AlertRuleModel: model.NewAlertRuleModel(db),
	})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(s)
	r.GET("/alerts", h.List)
	r.GET("/alerts/rules", h.RulesList)
	r.POST("/alerts/rules", h.RulesCreate)
	r.PUT("/alerts/rules/:id", h.RulesUpdate)
	r.DELETE("/alerts/rules/:id", h.RulesDelete)
	return r, s, db
}

func doAlertReq(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestRulesHandler_CreateRoundTripAndDefaults(t *testing.T) {
	r, _, db := newAlertRulesEnv(t)

	// 显式字段全量往返(含 enabled=false)。
	rec := doAlertReq(r, http.MethodPost, "/alerts/rules", `{
		"name":"CPU 爆表","description":"持续高负载","metric":"cpu.usagePercent",
		"operator":"gte","threshold":85.5,"forCount":3,"cooldownSeconds":120,
		"level":"critical","agentFilter":"agent-1","enabled":false
	}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var created RuleCreateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.NotZero(t, created.Item.ID)
	assert.Equal(t, "CPU 爆表", created.Item.Name)
	assert.Equal(t, "cpu.usagePercent", created.Item.Metric)
	assert.Equal(t, "gte", created.Item.Operator)
	assert.InDelta(t, 85.5, created.Item.Threshold, 0.001)
	assert.Equal(t, 3, created.Item.ForCount)
	assert.Equal(t, 120, created.Item.CooldownSeconds)
	assert.Equal(t, "critical", created.Item.Level)
	assert.Equal(t, "agent-1", created.Item.AgentFilter)
	// 注意:enabled=false 时 create 响应里 enabled 会被 gorm 回填为列默认值 true
	// (库中实际为 false),此处按落库结果断言,见报告。
	var stored model.AlertRule
	require.NoError(t, db.First(&stored, created.Item.ID).Error)
	assert.False(t, stored.Enabled)
	assert.Empty(t, created.Item.LastFiredAt)

	// 缺省字段:forCount→1、cooldownSeconds→300、level→warning、enabled→true。
	rec = doAlertReq(r, http.MethodPost, "/alerts/rules",
		`{"name":"内存高","metric":"memory.usedBytes","operator":"gt","threshold":100}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, 1, created.Item.ForCount)
	assert.Equal(t, 300, created.Item.CooldownSeconds)
	assert.Equal(t, model.AlertRuleLevelWarning, created.Item.Level)
	assert.True(t, created.Item.Enabled)

	// 列表过滤。
	listAndAssert := func(query string, want int, wantEnabled *bool) {
		t.Helper()
		rec := doAlertReq(r, http.MethodGet, "/alerts/rules"+query, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var resp RulesListResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Len(t, resp.Items, want, query)
		if wantEnabled != nil && want > 0 {
			assert.Equal(t, *wantEnabled, resp.Items[0].Enabled, query)
		}
	}
	listAndAssert("", 2, nil)
	listAndAssert("?metric=cpu.usagePercent", 1, nil)
	listAndAssert("?metric=custom.nope", 0, nil)
	enabledFalse, enabledTrue := false, true
	listAndAssert("?enabled=false", 1, &enabledFalse)
	listAndAssert("?enabled=true", 1, &enabledTrue)
	listAndAssert("?enabled=bogus", 2, nil) // 非法值不过滤

	// 持久化核对。
	rules, err := model.NewAlertRuleModel(db).List(context.Background(), model.ListAlertRulesOptions{})
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "cpu.usagePercent", rules[0].Metric)
}

func TestRulesHandler_CreateInvalidInput(t *testing.T) {
	r, _, _ := newAlertRulesEnv(t)
	cases := []struct {
		name string
		body string
	}{
		{"bad metric", `{"name":"x","metric":"cpu","operator":"gt","threshold":1}`},
		{"bad disk metric field", `{"name":"x","metric":"disk./data.bogus","operator":"gt","threshold":1}`},
		{"empty custom key", `{"name":"x","metric":"custom.","operator":"gt","threshold":1}`},
		{"bad operator", `{"name":"x","metric":"cpu.usagePercent","operator":"eq","threshold":1}`},
		{"bad level", `{"name":"x","metric":"cpu.usagePercent","operator":"gt","level":"fatal"}`},
		{"missing name", `{"metric":"cpu.usagePercent","operator":"gt"}`},
		{"missing metric", `{"name":"x","operator":"gt"}`},
		{"missing operator", `{"name":"x","metric":"cpu.usagePercent"}`},
		{"malformed json", `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doAlertReq(r, http.MethodPost, "/alerts/rules", tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

func TestRulesHandler_UpdateFlows(t *testing.T) {
	r, _, _ := newAlertRulesEnv(t)
	rec := doAlertReq(r, http.MethodPost, "/alerts/rules",
		`{"name":"cpu","metric":"cpu.usagePercent","operator":"gt","threshold":80,"level":"warning"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var created RuleCreateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id := fmt.Sprintf("%d", created.Item.ID)

	// 部分更新往返。
	rec = doAlertReq(r, http.MethodPut, "/alerts/rules/"+id,
		`{"threshold":95.5,"forCount":2,"cooldownSeconds":60,"enabled":false,"name":"cpu-2","description":"  升级  ","agentFilter":"agent-9"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var updated RuleUpdateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.InDelta(t, 95.5, updated.Item.Threshold, 0.001)
	assert.Equal(t, 2, updated.Item.ForCount)
	assert.Equal(t, 60, updated.Item.CooldownSeconds)
	assert.False(t, updated.Item.Enabled)
	assert.Equal(t, "cpu-2", updated.Item.Name)
	assert.Equal(t, "升级", updated.Item.Description)
	assert.Equal(t, "agent-9", updated.Item.AgentFilter)
	assert.Equal(t, "cpu.usagePercent", updated.Item.Metric) // 不可变字段不变

	// forCount<1 归一为 1。
	rec = doAlertReq(r, http.MethodPut, "/alerts/rules/"+id, `{"forCount":0}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Equal(t, 1, updated.Item.ForCount)

	// 空更新返回当前值。
	rec = doAlertReq(r, http.MethodPut, "/alerts/rules/"+id, `{}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Equal(t, "cpu-2", updated.Item.Name)

	// level 提供但非法 → 400。
	rec = doAlertReq(r, http.MethodPut, "/alerts/rules/"+id, `{"level":"fatal"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 无效/不存在 ID。
	rec = doAlertReq(r, http.MethodPut, "/alerts/rules/abc", `{"threshold":1}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doAlertReq(r, http.MethodPut, "/alerts/rules/0", `{"threshold":1}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doAlertReq(r, http.MethodPut, "/alerts/rules/999999", `{"threshold":1}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRulesHandler_DeleteFlow(t *testing.T) {
	r, _, db := newAlertRulesEnv(t)
	for _, body := range []string{
		`{"name":"a","metric":"cpu.usagePercent","operator":"gt","threshold":1}`,
		`{"name":"b","metric":"memory.usedBytes","operator":"gt","threshold":1}`,
	} {
		rec := doAlertReq(r, http.MethodPost, "/alerts/rules", body)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}

	rec := doAlertReq(r, http.MethodDelete, "/alerts/rules/999999", "")
	assert.Equal(t, http.StatusOK, rec.Code) // gorm 删除不存在的行不报错(当前行为)

	var list RulesListResponse
	rec = doAlertReq(r, http.MethodGet, "/alerts/rules", "")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Items, 2)
	id := fmt.Sprintf("%d", list.Items[0].ID)

	rec = doAlertReq(r, http.MethodDelete, "/alerts/rules/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"ok":true`)

	rec = doAlertReq(r, http.MethodGet, "/alerts/rules", "")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Len(t, list.Items, 1)

	rec = doAlertReq(r, http.MethodDelete, "/alerts/rules/abc", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	count := int64(0)
	require.NoError(t, db.Model(&model.AlertRule{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestRulesService_NilModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	ctx := context.Background()
	_, err := s.RulesList(ctx, &RulesListRequest{})
	assert.ErrorContains(t, err, "告警规则模型未初始化")
	_, err = s.RulesCreate(ctx, &RuleCreateRequest{Name: "x", Metric: "cpu.usagePercent", Operator: "gt"})
	assert.ErrorContains(t, err, "告警规则模型未初始化")
	_, err = s.RulesUpdate(ctx, 1, &RuleUpdateRequest{})
	assert.ErrorContains(t, err, "告警规则模型未初始化")
	err = s.RulesDelete(ctx, 1)
	assert.ErrorContains(t, err, "告警规则模型未初始化")
}

func TestAlertService_ListLevelStatusFilters(t *testing.T) {
	_, s, db := newAlertRulesEnv(t)
	ctx := context.Background()
	am := model.NewAlertModel(db)
	seed := []model.Alert{
		{AlertID: "a-crit", Type: "db", Level: "critical", Status: "firing", Message: "m1"},
		{AlertID: "a-warn1", Type: "db", Level: "warning", Status: "firing", Message: "m2"},
		{AlertID: "a-warn2", Type: "db", Level: "warning", Status: "resolved", Message: "m3"},
	}
	for i := range seed {
		require.NoError(t, am.Create(ctx, &seed[i]))
	}

	list := func(level, status string) (*AlertsListResponse, error) {
		return s.List(ctx, &AlertsListRequest{Page: 1, PageSize: 20, Level: level, Status: status})
	}

	resp, err := list("", "")
	require.NoError(t, err)
	assert.EqualValues(t, 3, resp.Total)

	resp, err = list("critical", "")
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "a-crit", resp.Items[0].Id)

	resp, err = list("warning", "firing")
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "a-warn1", resp.Items[0].Id)
	assert.Equal(t, "firing", resp.Items[0].Status)

	// 过滤空白与 details 透传。
	resp, err = list(" warning ", "")
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)

	detailed := &model.Alert{AlertID: "a-detail", Type: "db", Level: "info", Status: "firing", Details: datatypes.JSONMap{"k": "v"}}
	require.NoError(t, am.Create(ctx, detailed))
	resp, err = list("info", "")
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.NotNil(t, resp.Items[0].Details)
}

func TestSilenceService_LifecycleAndValidation(t *testing.T) {
	_, s, db := newAlertRulesEnv(t)
	ctx := context.Background()
	am := model.NewAlertModel(db)
	alert := &model.Alert{AlertID: "sil-target", Type: "db_monitor", Level: "warning", Status: "firing", Message: "m"}
	require.NoError(t, am.Create(ctx, alert))

	// 负 duration 归一为 60 分钟。
	require.NoError(t, s.Silence(ctx, &AlertSilenceRequest{ID: "sil-target", Duration: -5, Reason: "  发布窗口  "}))
	var silences []model.AlertSilence
	require.NoError(t, db.Find(&silences).Error)
	require.Len(t, silences, 1)
	assert.Equal(t, 60, silences[0].DurationMinute)
	assert.Equal(t, "发布窗口", silences[0].Reason)
	assert.WithinDuration(t, time.Now().Add(60*time.Minute), silences[0].ExpiresAt, 2*time.Minute)

	// 列表带出 alertType。
	resp, err := s.SilencesList(ctx, &SilencesListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "db_monitor", resp.Items[0].AlertType)
	assert.NotEmpty(t, resp.Items[0].EndAt)

	// 删除。
	require.NoError(t, s.SilenceDelete(ctx, &SilenceDeleteRequest{ID: fmt.Sprintf("%d", silences[0].ID)}))
	resp, err = s.SilencesList(ctx, &SilencesListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	// 校验分支。
	err = s.Silence(ctx, &AlertSilenceRequest{ID: "   "})
	assert.ErrorContains(t, err, "告警ID不能为空")
	err = s.Silence(ctx, &AlertSilenceRequest{ID: "no-such-alert"})
	assert.Error(t, err)
	err = s.SilenceDelete(ctx, &SilenceDeleteRequest{ID: "abc"})
	assert.ErrorContains(t, err, "静默ID格式不正确")
}
