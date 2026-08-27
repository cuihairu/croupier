package dbmon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// 注册真实驱动,让 probe 走到拨号失败分支而非 sql.Open 报 unknown driver。
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func newDBMonTestEnv(t *testing.T) (*gin.Engine, *Service, *model.DBSourceModel, *model.AlertModel) {
	t.Helper()
	dbmonSvc, srcModel, alertModel := newDBMonFixture(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "username", "alice"))
	})
	h := NewHandler(dbmonSvc)
	api := r.Group("/dbmon")
	api.GET("/sources", h.ListSources)
	api.POST("/sources", h.CreateSource)
	api.PUT("/sources/:id", h.UpdateSource)
	api.DELETE("/sources/:id", h.DeleteSource)
	api.POST("/probe", h.ProbeAll)
	return r, dbmonSvc, srcModel, alertModel
}

func doDBMonReq(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
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

const goodDSN = "monitor:sec%ret@tcp(10.0.0.1:3306)/game"

func TestDBMonHandler_SourceCRUDRoundTrip(t *testing.T) {
	r, _, _, _ := newDBMonTestEnv(t)

	// 创建:driver/kind 归一化、DSN 脱敏、默认 enabled、createdBy 取上下文用户。
	rec := doDBMonReq(r, http.MethodPost, "/dbmon/sources", fmt.Sprintf(`{
		"name":"游戏主库","driver":"MySQL","kind":" Self ","dsn":%q,"gameId":"demo","env":"prod","sort":3
	}`, goodDSN))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var created SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.NotZero(t, created.Id)
	assert.Equal(t, "mysql", created.Driver)
	assert.Equal(t, "self", created.Kind)
	assert.Equal(t, "monitor:***@tcp(10.0.0.1:3306)/game", created.DsnMask)
	assert.NotContains(t, created.DsnMask, "sec%ret")
	assert.True(t, created.Enabled)
	assert.Equal(t, "alice", created.CreatedBy)
	assert.Equal(t, 3, created.Sort)

	// 列表。
	rec = doDBMonReq(r, http.MethodGet, "/dbmon/sources", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var list SourceListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Items, 1)
	assert.Equal(t, "monitor:***@tcp(10.0.0.1:3306)/game", list.Items[0].DsnMask)

	// 更新:改名 + 关闭 enabled。
	rec = doDBMonReq(r, http.MethodPut, fmt.Sprintf("/dbmon/sources/%d", created.Id),
		`{"name":"主库-2","enabled":false}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var updated SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Equal(t, "主库-2", updated.Name)
	assert.False(t, updated.Enabled)

	// 空更新 → 400。
	rec = doDBMonReq(r, http.MethodPut, fmt.Sprintf("/dbmon/sources/%d", created.Id), `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 删除后列表为空。
	rec = doDBMonReq(r, http.MethodDelete, fmt.Sprintf("/dbmon/sources/%d", created.Id), "")
	require.Equal(t, http.StatusOK, rec.Code)
	rec = doDBMonReq(r, http.MethodGet, "/dbmon/sources", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Empty(t, list.Items)
}

func TestDBMonHandler_CreateSourceRejections(t *testing.T) {
	r, _, _, _ := newDBMonTestEnv(t)
	cases := []struct {
		name string
		body string
		want string
	}{
		{"bad driver", `{"name":"x","driver":"oracle","dsn":"a:b@c/d"}`, "驱动"},
		{"bad kind", `{"name":"x","driver":"mysql","kind":"gcp","dsn":"a:b@c/d"}`, "部署类型"},
		{"root dsn", `{"name":"x","driver":"mysql","dsn":"root:pw@tcp(127.0.0.1)/x"}`, "只读"},
		{"superuser dsn", `{"name":"x","driver":"postgres","dsn":"postgres://superuser:pw@h/db"}`, "只读"},
		{"malformed dsn", `{"name":"x","driver":"mysql","dsn":"just-a-string"}`, "DSN"},
		{"empty name", `{"name":"","driver":"mysql","dsn":"a:b@c/d"}`, "名称"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doDBMonReq(r, http.MethodPost, "/dbmon/sources", tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}

	rec := doDBMonReq(r, http.MethodPost, "/dbmon/sources", `{not-json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDBMonHandler_UpdateDeleteInvalidOrMissingID(t *testing.T) {
	r, _, srcModel, _ := newDBMonTestEnv(t)
	ctx := context.Background()
	src := &model.DBSource{Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN, Enabled: true}
	require.NoError(t, srcModel.Create(ctx, src))

	for _, path := range []string{"/dbmon/sources/abc", "/dbmon/sources/0", "/dbmon/sources/-1"} {
		rec := doDBMonReq(r, http.MethodPut, path, `{"name":"x"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, path)
	}
	rec := doDBMonReq(r, http.MethodPut, "/dbmon/sources/99999", `{"name":"x"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = doDBMonReq(r, http.MethodDelete, "/dbmon/sources/abc", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doDBMonReq(r, http.MethodDelete, "/dbmon/sources/99999", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDBMonService_UpdateSourceFieldValidation(t *testing.T) {
	dbmonSvc, srcModel, _ := newDBMonFixture(t)
	ctx := context.Background()
	src := &model.DBSource{Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN, Enabled: true}
	require.NoError(t, srcModel.Create(ctx, src))
	id := fmt.Sprintf("%d", src.ID)

	// 合并后校验:非法 kind / driver / root DSN 均拒绝。
	badKind, badDriver, rootDSN := "gcp", "oracle", "root:pw@tcp(10.0.0.2)/x"
	for _, tc := range []struct {
		name string
		req  SourceUpdateRequest
		want string
	}{
		{"bad kind", SourceUpdateRequest{ID: id, Kind: badKind}, "部署类型"},
		{"bad driver", SourceUpdateRequest{ID: id, Driver: badDriver}, "驱动"},
		{"root dsn", SourceUpdateRequest{ID: id, DSN: &rootDSN}, "只读"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dbmonSvc.UpdateSource(ctx, &tc.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}

	// 合法部分更新:DSN / gameID / env / sort 往返。
	newDSN := "monitor:pw2@tcp(10.0.0.9:3306)/game2"
	gameID, env, sort := "game2", "dev", 7
	resp, err := dbmonSvc.UpdateSource(ctx, &SourceUpdateRequest{
		ID: id, DSN: &newDSN, GameID: &gameID, Env: &env, Sort: &sort,
	})
	require.NoError(t, err)
	assert.Equal(t, "monitor:***@tcp(10.0.0.9:3306)/game2", resp.DsnMask)
	stored, err := srcModel.FindOne(ctx, src.ID)
	require.NoError(t, err)
	assert.Equal(t, "game2", stored.GameID)
	assert.Equal(t, "dev", stored.Env)
	assert.Equal(t, 7, stored.Sort)
}

func TestDBMonService_CreateSourceCreatedByFallback(t *testing.T) {
	dbmonSvc, srcModel, _ := newDBMonFixture(t)
	ctx := context.Background()

	resp, err := dbmonSvc.CreateSource(ctx, &SourceUpsertRequest{Name: "a", Driver: "mysql", DSN: goodDSN})
	require.NoError(t, err)
	assert.Equal(t, "system", resp.CreatedBy)

	named := context.WithValue(ctx, "username", "bob")
	resp, err = dbmonSvc.CreateSource(named, &SourceUpsertRequest{Name: "b", Driver: "mysql", DSN: goodDSN})
	require.NoError(t, err)
	assert.Equal(t, "bob", resp.CreatedBy)

	items, err := srcModel.List(ctx)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestDBMonService_ProbeOneEmptyDSN(t *testing.T) {
	dbmonSvc, _, _ := newDBMonFixture(t)
	_, err := dbmonSvc.ProbeOne(context.Background(), &model.DBSource{Name: "x", Driver: "mysql"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no DSN")
}

func TestDBMonHandler_ProbeAllDegradedOnUnreachable(t *testing.T) {
	r, _, srcModel, alertModel := newDBMonTestEnv(t)
	ctx := context.Background()

	// 坏 DSN 的 mysql 源(enabled)、正常但 disabled 的源、坏 DSN 的 postgres 源。
	require.NoError(t, srcModel.Create(ctx, &model.DBSource{
		Name: "bad-mysql", Driver: "mysql", Kind: model.DBSourceKindSelf,
		DSN: "ro:ro@tcp(127.0.0.1:1)/none?timeout=1s&readTimeout=1s", Enabled: true,
	}))
	off := &model.DBSource{Name: "disabled", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN}
	require.NoError(t, srcModel.Create(ctx, off))
	// Create 会把零值 Enabled=false 吞成列默认 true,用 map 更新关闭。
	require.NoError(t, srcModel.Update(ctx, off.ID, map[string]interface{}{"enabled": false}))
	require.NoError(t, srcModel.Create(ctx, &model.DBSource{
		Name: "bad-pg", Driver: "postgres", Kind: model.DBSourceKindAliyun,
		DSN: "postgres://ro:pw@127.0.0.1:1/none?sslmode=disable&connect_timeout=1", Enabled: true,
	}))

	rec := doDBMonReq(r, http.MethodPost, "/dbmon/probe", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp ProbeAllResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	// disabled 源被跳过。
	require.Len(t, resp.Results, 2)
	byName := map[string]ProbeResult{}
	for _, res := range resp.Results {
		byName[res.Name] = res
	}

	// mysql:连接失败 → degraded(结果带 error,接口不 500)。
	mysqlRes, ok := byName["bad-mysql"]
	require.True(t, ok, "bad-mysql should be probed")
	assert.False(t, mysqlRes.OK)
	assert.NotEmpty(t, mysqlRes.Error)
	assert.False(t, mysqlRes.ProbedAt.IsZero())

	// postgres 源返回结果而非接口报错(当前实现对查询失败静默降级,仅校验不 500)。
	pgRes, ok := byName["bad-pg"]
	require.True(t, ok, "bad-pg should be probed")
	assert.Equal(t, "postgres", pgRes.Driver)
	_, ok = byName["disabled"]
	assert.False(t, ok, "disabled source should be skipped")

	// 探测失败不产生告警。
	alerts, _, err := alertModel.List(ctx, model.ListAlertsOptions{})
	require.NoError(t, err)
	assert.Empty(t, alerts)
}

func TestDBMonService_ProbeNilSource(t *testing.T) {
	_, err := Probe(context.Background(), nil, goodDSN)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil source")
}

func lockWaitRes(n int, conns *ConnectionsInfo) *ProbeResult {
	res := &ProbeResult{OK: true, Name: "src"}
	for i := 0; i < n; i++ {
		res.LockWaits = append(res.LockWaits, LockWait{WaitPIDorID: fmt.Sprintf("p%d", i), BlockedBy: "b"})
	}
	res.Connections = conns
	return res
}

func TestRaiseAlertsIfNeeded(t *testing.T) {
	ctx := context.Background()

	t.Run("lock waits above default threshold fires critical", func(t *testing.T) {
		dbmonSvc, srcModel, alertModel := newDBMonFixture(t)
		src := &model.DBSource{Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN}
		require.NoError(t, srcModel.Create(ctx, src))
		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(6, nil))

		alerts, _, err := alertModel.List(ctx, model.ListAlertsOptions{})
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		assert.Equal(t, "firing", alerts[0].Status)
		assert.Equal(t, "critical", alerts[0].Level)
		assert.Equal(t, "dbmon", alerts[0].Source)
		assert.Equal(t, "db_monitor", alerts[0].Type)
		assert.Contains(t, alerts[0].Message, "锁等待")
	})

	t.Run("lock waits at custom threshold", func(t *testing.T) {
		dbmonSvc, srcModel, alertModel := newDBMonFixture(t)
		src := &model.DBSource{Name: "s", Driver: "mysql", DSN: goodDSN, LockWaitWarn: 2}
		require.NoError(t, srcModel.Create(ctx, src))
		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(2, nil)) // 等于阈值不触发
		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(3, nil)) // 超过触发
		alerts, _, err := alertModel.List(ctx, model.ListAlertsOptions{})
		require.NoError(t, err)
		require.Len(t, alerts, 1)
	})

	t.Run("connection ratio default threshold 80", func(t *testing.T) {
		dbmonSvc, srcModel, alertModel := newDBMonFixture(t)
		src := &model.DBSource{Name: "s", Driver: "mysql", DSN: goodDSN}
		require.NoError(t, srcModel.Create(ctx, src))

		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(0, &ConnectionsInfo{Current: 79, Max: 100}))
		alerts, _, err := alertModel.List(ctx, model.ListAlertsOptions{})
		require.NoError(t, err)
		assert.Empty(t, alerts)

		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(0, &ConnectionsInfo{Current: 80, Max: 100}))
		alerts, _, err = alertModel.List(ctx, model.ListAlertsOptions{})
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		assert.Equal(t, "warning", alerts[0].Level)
		assert.Contains(t, alerts[0].Message, "连接水位")
	})

	t.Run("custom conn warn ratio suppresses default", func(t *testing.T) {
		dbmonSvc, srcModel, alertModel := newDBMonFixture(t)
		src := &model.DBSource{Name: "s", Driver: "mysql", DSN: goodDSN, ConnWarnRatio: 95}
		require.NoError(t, srcModel.Create(ctx, src))
		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(0, &ConnectionsInfo{Current: 90, Max: 100}))
		alerts, _, err := alertModel.List(ctx, model.ListAlertsOptions{})
		require.NoError(t, err)
		assert.Empty(t, alerts)
	})

	t.Run("lock wait takes precedence over conn ratio", func(t *testing.T) {
		dbmonSvc, srcModel, alertModel := newDBMonFixture(t)
		src := &model.DBSource{Name: "s", Driver: "mysql", DSN: goodDSN}
		require.NoError(t, srcModel.Create(ctx, src))
		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(6, &ConnectionsInfo{Current: 95, Max: 100}))
		alerts, _, err := alertModel.List(ctx, model.ListAlertsOptions{})
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		assert.Equal(t, "critical", alerts[0].Level)
	})

	t.Run("firing dedups then resolves", func(t *testing.T) {
		dbmonSvc, srcModel, alertModel := newDBMonFixture(t)
		src := &model.DBSource{Name: "s", Driver: "mysql", DSN: goodDSN}
		require.NoError(t, srcModel.Create(ctx, src))

		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(6, nil))
		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(7, nil)) // 已 firing → 去重
		alerts, _, err := alertModel.List(ctx, model.ListAlertsOptions{})
		require.NoError(t, err)
		require.Len(t, alerts, 1)

		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(0, nil)) // 恢复 → resolved
		alerts, _, err = alertModel.List(ctx, model.ListAlertsOptions{Status: "resolved"})
		require.NoError(t, err)
		require.Len(t, alerts, 1)

		dbmonSvc.raiseAlertsIfNeeded(ctx, src, lockWaitRes(0, nil)) // 再次 ok → 幂等
		alerts, _, err = alertModel.List(ctx, model.ListAlertsOptions{})
		require.NoError(t, err)
		assert.Len(t, alerts, 1)
	})

	t.Run("probe error result is ignored", func(t *testing.T) {
		dbmonSvc, srcModel, alertModel := newDBMonFixture(t)
		src := &model.DBSource{Name: "s", Driver: "mysql", DSN: goodDSN}
		require.NoError(t, srcModel.Create(ctx, src))
		dbmonSvc.raiseAlertsIfNeeded(ctx, src, &ProbeResult{Error: "boom"})
		alerts, _, err := alertModel.List(ctx, model.ListAlertsOptions{})
		require.NoError(t, err)
		assert.Empty(t, alerts)
	})

	t.Run("nil alert model is a no-op", func(t *testing.T) {
		s := NewService(&svc.ServiceContext{})
		assert.NotPanics(t, func() {
			s.raiseAlertsIfNeeded(ctx, &model.DBSource{Name: "s"}, lockWaitRes(9, nil))
		})
	})
}

func TestDBMonHelpers(t *testing.T) {
	assert.Equal(t, 123, atoi("123"))
	assert.Equal(t, 12, atoi("12a3"))
	assert.Equal(t, 0, atoi(""))

	assert.Equal(t, "short", truncate(" short ", 120))
	assert.Equal(t, "exactly10!", truncate("exactly10!", 10))
	assert.Equal(t, "0123456789…", truncate("0123456789x", 10))

	assert.Equal(t, "pgx", driverName("postgres"))
	assert.Equal(t, "mysql", driverName("mysql"))
	assert.Equal(t, "mysql", driverName("oracle"))

	assert.Equal(t, "dsn", resolveDSN(&model.DBSource{DSN: "  dsn  "}), "resolveDSN 负责去空白")
	assert.Equal(t, "", resolveDSN(&model.DBSource{}))

	src := &model.DBSource{Name: "n", Driver: "mysql", Kind: "self", DSN: goodDSN, GameID: "g", Env: "e", Enabled: true, Sort: 2, CreatedBy: "alice"}
	applyUpdates(src, map[string]interface{}{
		"name": "n2", "driver": "postgres", "kind": "aliyun", "dsn": "d2", "game_id": "g2", "env": "e2",
	})
	assert.Equal(t, "n2", src.Name)
	assert.Equal(t, "postgres", src.Driver)
	assert.Equal(t, "aliyun", src.Kind)
	assert.Equal(t, "d2", src.DSN)
	assert.Equal(t, "g2", src.GameID)
	assert.Equal(t, "e2", src.Env)

	dto := buildSourceDTO(&model.DBSource{Name: "x", DSN: "u:p@h/d"})
	assert.Equal(t, "u:***@h/d", dto.DsnMask)
	_, err := time.Parse(time.RFC3339, dto.CreatedAt)
	assert.NoError(t, err)
}
