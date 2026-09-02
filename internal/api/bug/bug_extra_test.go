// 覆盖目标：handler 各入口的绑定错误/Get/Delete 成功路径、service Get/Update 全字段
// 与非法枚举、List/Find 底层错误、normalizeSource/currentUsername/parseBugLinks/
// deriveBugLinkTitle/decodeBugLinks/toInt64/firstStackLine/truncateRunes 纯函数，
// 以及 ReportCrash 聚合分支（标题截断/平台回退/nil Extra 计数/查询错误）。
package bug

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"unicode/utf8"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBugGet_FoundAndNotFound(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	require.NoError(t, db.Create(&model.Bug{Title: "查询我", Status: model.BugStatusTriage}).Error)

	c, w := bugRequest(http.MethodGet, "/bugs/1", "")
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	h.Get(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "查询我")

	// 不存在 → 404（gorm.ErrRecordNotFound 统一映射）。
	c, w = bugRequest(http.MethodGet, "/bugs/99", "")
	c.Params = gin.Params{{Key: "id", Value: "99"}}
	h.Get(c)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 非数字 id → 400。
	c, w = bugRequest(http.MethodGet, "/bugs/abc", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.Get(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBugList_BindError(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	// page=abc 触发 ShouldBindQuery 失败（分支已覆盖）。注意：strconv 绑定错误
	// 未被 response.Error 识别为 400，实际映射 500——疑似契约偏差，见交付报告。
	c, w := bugRequest(http.MethodGet, "/bugs?page=abc", "")
	h.List(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.String())
}

func TestBugList_DbError(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	require.NoError(t, db.Migrator().DropTable("bugs"))
	c, w := bugRequest(http.MethodGet, "/bugs", "")
	h.List(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestBugCreate_BindError(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	c, w := bugRequest(http.MethodPost, "/bugs", `{"title":`)
	h.Create(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBugUpdate_JSONBindError(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	c, w := bugRequest(http.MethodPut, "/bugs/1", `{oops`)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	h.Update(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBugReportCrash_BindError(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	c, w := bugRequest(http.MethodPost, "/bugs/crash", `{`)
	h.ReportCrash(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBugDelete_Success(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	require.NoError(t, db.Create(&model.Bug{Title: "删除我", Status: model.BugStatusTriage}).Error)

	c, w := bugRequest(http.MethodDelete, "/bugs/1", "")
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	h.Delete(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "删除成功")

	var count int64
	db.Model(&model.Bug{}).Count(&count)
	assert.Zero(t, count)

	// 删除不存在 → 404（RowsAffected==0 → ErrRecordNotFound）。
	c, w = bugRequest(http.MethodDelete, "/bugs/9", "")
	c.Params = gin.Params{{Key: "id", Value: "9"}}
	h.Delete(c)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 非法 id → 400。
	c, w = bugRequest(http.MethodDelete, "/bugs/x", "")
	c.Params = gin.Params{{Key: "id", Value: "x"}}
	h.Delete(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- service 层直测 ----

func newSvc(db *gorm.DB) *Service {
	return NewService(&svc.ServiceContext{BugModel: model.NewBugModel(db)})
}

func TestBugService_Get(t *testing.T) {
	db := newBugTestDB(t)
	s := newSvc(db)
	require.NoError(t, db.Create(&model.Bug{Title: "svc-get", Status: model.BugStatusTriage}).Error)

	resp, err := s.Get(context.Background(), &BugGetRequest{ID: "1"})
	require.NoError(t, err)
	assert.Equal(t, "svc-get", resp.Title)

	_, err = s.Get(context.Background(), &BugGetRequest{ID: "42"})
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	_, err = s.Get(context.Background(), &BugGetRequest{ID: "bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "缺陷 ID")
}

func TestBugService_UpdateAllFields(t *testing.T) {
	db := newBugTestDB(t)
	s := newSvc(db)
	require.NoError(t, db.Create(&model.Bug{Title: "origin", Status: model.BugStatusTriage}).Error)

	content, status := "新内容", model.BugStatusFixing
	severity, priority, assignee := model.BugSeverityMajor, "P1", "dev"
	steps, repro := "1. 复现", "sometimes"
	affects, fix, platform := "1.0.0", "1.1.0", "android"
	resp, err := s.Update(context.Background(), &BugUpdateRequest{
		ID:              "1",
		Title:           "新标题",
		Content:         &content,
		Status:          &status,
		Severity:        &severity,
		Priority:        &priority,
		Assignee:        &assignee,
		Steps:           &steps,
		Reproducibility: &repro,
		AffectsVersion:  &affects,
		FixVersion:      &fix,
		Platform:        &platform,
		Links:           json.RawMessage(`[{"url":"https://github.com/a/b/pull/9","kind":"github_pr"}]`),
	})
	require.NoError(t, err)
	assert.Equal(t, "新标题", resp.Title)
	assert.Equal(t, content, resp.Content)
	assert.Equal(t, model.BugStatusFixing, resp.Status)
	assert.Equal(t, model.BugSeverityMajor, resp.Severity)
	assert.Equal(t, "P1", resp.Priority)
	assert.Equal(t, "dev", resp.Assignee)
	assert.Equal(t, steps, resp.Steps)
	assert.Equal(t, repro, resp.Reproducibility)
	assert.Equal(t, affects, resp.AffectsVersion)
	assert.Equal(t, fix, resp.FixVersion)
	assert.Equal(t, platform, resp.Platform)
	require.Len(t, resp.Links, 1)
	assert.Equal(t, "a/b#9", resp.Links[0].Title) // 标题自动推导
}

func TestBugService_UpdateInvalidValues(t *testing.T) {
	db := newBugTestDB(t)
	s := newSvc(db)
	require.NoError(t, db.Create(&model.Bug{Title: "t", Status: model.BugStatusTriage}).Error)

	str := func(v string) *string { return &v }
	cases := []struct {
		name string
		req  BugUpdateRequest
		want string
	}{
		{"bad status", BugUpdateRequest{ID: "1", Status: str("bogus")}, "状态"},
		{"bad severity", BugUpdateRequest{ID: "1", Severity: str("bogus")}, "严重度"},
		{"bad repro", BugUpdateRequest{ID: "1", Reproducibility: str("bogus")}, "复现率"},
		{"bad links kind", BugUpdateRequest{ID: "1", Links: json.RawMessage(`[{"url":"https://a.b","kind":"nope"}]`)}, "kind"},
		{"bad links json", BugUpdateRequest{ID: "1", Links: json.RawMessage(`[{`)}, "links"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Update(context.Background(), &tc.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}

	// 无任何可更新字段。
	_, err := s.Update(context.Background(), &BugUpdateRequest{ID: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "更新的字段")

	// 非法 id。
	_, err = s.Update(context.Background(), &BugUpdateRequest{ID: "zz"})
	require.Error(t, err)

	// 目标不存在：Update 0 行后 FindOne 404。
	_, err = s.Update(context.Background(), &BugUpdateRequest{ID: "77", Title: "x"})
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestBugService_Delete(t *testing.T) {
	db := newBugTestDB(t)
	s := newSvc(db)
	require.NoError(t, db.Create(&model.Bug{Title: "del", Status: model.BugStatusTriage}).Error)
	require.NoError(t, s.Delete(context.Background(), &BugDeleteRequest{ID: "1"}))
	require.Error(t, s.Delete(context.Background(), &BugDeleteRequest{ID: "0"}))
}

func TestBugCreate_SourceNormalization(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	for _, src := range []string{"player", "ticket", "qa"} {
		c, w := bugRequest(http.MethodPost, "/bugs", fmt.Sprintf(`{"title":"s-%s","source":%q}`, src, src))
		h.Create(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	var bugs []model.Bug
	require.NoError(t, db.Order("id").Find(&bugs).Error)
	require.Len(t, bugs, 3)
	assert.Equal(t, "player", bugs[0].Source)
	assert.Equal(t, "ticket", bugs[1].Source)
	assert.Equal(t, "internal", bugs[2].Source) // 未知来源归一为 internal
}

// ---- 纯函数直测 ----

func TestNormalizeSource(t *testing.T) {
	assert.Equal(t, "player", normalizeSource(" player "))
	assert.Equal(t, "ticket", normalizeSource("ticket"))
	assert.Equal(t, "internal", normalizeSource("whatever"))
	assert.Equal(t, "internal", normalizeSource(""))
}

func TestCurrentUsername(t *testing.T) {
	ctx := context.WithValue(context.Background(), "username", "alice")
	assert.Equal(t, "alice", currentUsername(ctx))
	assert.Equal(t, "system", currentUsername(context.Background()))
}

func TestParseBugLinks_Direct(t *testing.T) {
	out, err := parseBugLinks(nil)
	require.NoError(t, err)
	assert.Nil(t, out)

	out, err = parseBugLinks(json.RawMessage("null"))
	require.NoError(t, err)
	assert.Nil(t, out)

	_, err = parseBugLinks(json.RawMessage(`[{`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "links")

	// 标题空白 → 从 URL 推导；已有标题 → 仅 trim。
	out, err = parseBugLinks(json.RawMessage(`[{"url":"https://e.io/x","kind":"other","title":"  "}]`))
	require.NoError(t, err)
	var links []model.BugLink
	require.NoError(t, json.Unmarshal(out, &links))
	assert.Equal(t, "e.io/x", links[0].Title)

	out, err = parseBugLinks(json.RawMessage(`[{"url":"https://e.io/x","kind":"other","title":"  keep  "}]`))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(out, &links))
	assert.Equal(t, "keep", links[0].Title)
}

func TestDeriveBugLinkTitle(t *testing.T) {
	assert.Equal(t, "o/r#42", deriveBugLinkTitle("https://github.com/o/r/issues/42", model.BugLinkGithubIssue))
	assert.Equal(t, "o/r#7", deriveBugLinkTitle("https://github.com/o/r/pull/7", model.BugLinkGithubPR))
	// 非 github kind 不做 #number 推导。
	assert.Equal(t, "github.com/o/r/issues/42", deriveBugLinkTitle("https://github.com/o/r/issues/42", model.BugLinkOther))
	// github URL 但路径不匹配 → host+path。
	assert.Equal(t, "github.com/o/r", deriveBugLinkTitle("https://github.com/o/r", model.BugLinkGithubIssue))
	// path 为 "/" 或空 → 仅 host。
	assert.Equal(t, "example.com", deriveBugLinkTitle("https://example.com/", model.BugLinkOther))
	assert.Equal(t, "example.com", deriveBugLinkTitle("https://example.com", model.BugLinkOther))
	// 无 host → 原样返回。
	assert.Equal(t, "not a url", deriveBugLinkTitle("not a url", model.BugLinkOther))
	// 解析失败（非法转义）→ 原样返回。
	assert.Equal(t, "http://h/%zz", deriveBugLinkTitle("http://h/%zz", model.BugLinkOther))
}

func TestDecodeBugLinks(t *testing.T) {
	assert.Nil(t, decodeBugLinks(nil))
	assert.Nil(t, decodeBugLinks(model.JSON(`[{`)))
	out := decodeBugLinks(model.JSON(`[{"url":"https://e.io","kind":"other","title":"  t  "}]`))
	require.Len(t, out, 1)
	assert.Equal(t, "t", out[0].Title)
}

func TestToInt64(t *testing.T) {
	v, err := toInt64(int64(5))
	require.NoError(t, err)
	assert.Equal(t, int64(5), v)

	v, err = toInt64(7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), v)

	v, err = toInt64(2.9)
	require.NoError(t, err)
	assert.Equal(t, int64(2), v)

	v, err = toInt64(json.Number("42"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), v)

	v, err = toInt64("17")
	require.NoError(t, err)
	assert.Equal(t, int64(17), v)

	_, err = toInt64("x")
	assert.Error(t, err)

	_, err = toInt64(true)
	assert.Error(t, err)
}

func TestFirstStackLine(t *testing.T) {
	assert.Equal(t, "first", firstStackLine("first\nsecond"))
	assert.Equal(t, "single", firstStackLine("single"))
	// 首字符即换行：i==0，返回整串。
	assert.Equal(t, "\nrest", firstStackLine("\nrest"))
}

func TestTruncateRunes(t *testing.T) {
	assert.Equal(t, "short", truncateRunes("short", 10))
	long := ""
	for range 70 {
		long += "崩"
	}
	got := truncateRunes(long, 60)
	assert.Equal(t, 60, utf8.RuneCountInString(got))
}

// ---- ReportCrash 分支 ----

func TestReportCrash_TitleAndMetadataBranches(t *testing.T) {
	h, db := newCrashHandler(t)

	// 无 message、首行超 60 rune → 标题取首行截断；platform 非法回退空；
	// playerId/appVersion 缺省不写 extra。
	longLine := ""
	for range 70 {
		longLine += "E"
	}
	stack := longLine + "\nframe two"
	c, w := crashReq(http.MethodPost, "/bugs/crash", fmt.Sprintf(`{"gameId":"demo","platform":"console","stack":%q}`, stack))
	h.ReportCrash(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var stored model.Bug
	require.NoError(t, db.First(&stored).Error)
	assert.Contains(t, stored.Title, "崩溃: ")
	assert.True(t, utf8.RuneCountInString(stored.Title) <= 65+utf8.RuneCountInString("崩溃: "))
	assert.NotContains(t, stored.Title, "frame two")
	assert.Empty(t, stored.Platform)
	assert.NotContains(t, stored.Extra, crashLastPlayerKey)
	assert.NotContains(t, stored.Extra, "appVersion")

	// message 提供时优先作为标题。
	c, w = crashReq(http.MethodPost, "/bugs/crash", `{"gameId":"g2","message":"自定义标题","stack":"boom"}`)
	h.ReportCrash(c)
	require.Equal(t, http.StatusOK, w.Code)
	var second model.Bug
	require.NoError(t, db.Where("game_id = ?", "g2").First(&second).Error)
	assert.Contains(t, second.Title, "自定义标题")
}

func TestReportCrash_BumpNilExtra(t *testing.T) {
	h, db := newCrashHandler(t)
	stack := "lua: /game/x.lua:1: boom"
	fp := fingerprintStack(stack)
	// 预置同指纹但 Extra 为 nil 的 bug：bump 需自建 map。
	require.NoError(t, db.Create(&model.Bug{
		Title: "预置", Status: model.BugStatusTriage, GameID: "demo", Env: "prod",
		CrashFingerprint: fp,
	}).Error)

	c, w := crashReq(http.MethodPost, "/bugs/crash",
		fmt.Sprintf(`{"gameId":"demo","env":"prod","stack":%q,"playerId":"p-9"}`, stack))
	h.ReportCrash(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var stored model.Bug
	require.NoError(t, db.Where("crash_fingerprint = ?", fp).First(&stored).Error)
	// Extra 为 nil 时计数从 0 起步：首次 bump 得 1。
	countVal, err := toInt64(stored.Extra[crashCountKey])
	require.NoError(t, err)
	assert.Equal(t, int64(1), countVal)
	assert.Equal(t, "p-9", stored.Extra[crashLastPlayerKey])
}

func TestReportCrash_FindError(t *testing.T) {
	h, db := newCrashHandler(t)
	require.NoError(t, db.Migrator().DropTable("bugs"))
	c, w := crashReq(http.MethodPost, "/bugs/crash", `{"gameId":"demo","stack":"boom"}`)
	h.ReportCrash(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
