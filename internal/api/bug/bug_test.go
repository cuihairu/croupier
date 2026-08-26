package bug

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var bugDBSeq uint64

func newBugTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("bug_%d", atomic.AddUint64(&bugDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newBugHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{BugModel: model.NewBugModel(db)}
	return NewHandler(NewService(svcCtx))
}

func bugRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	c.Request = httptest.NewRequest(method, target, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestBugCRUD_RoundTrip(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)

	// Create with full context + links.
	body := `{
		"title": "iOS 充值弹窗卡死",
		"content": "点击充值后弹窗无法关闭",
		"severity": "critical",
		"priority": "urgent",
		"status": "triage",
		"assignee": "qa",
		"gameId": "demo",
		"env": "prod",
		"serverId": "s-3",
		"platform": "ios",
		"device": "iPhone 15 Pro",
		"os": "iOS 18.1",
		"steps": "1. 进入商店\n2. 点击充值",
		"reproducibility": "always",
		"affectsVersion": "1.4.2",
		"fixVersion": "1.4.3",
		"playerId": "p-1",
		"links": [
			{"url": "https://github.com/acme/game/issues/42", "kind": "github_issue"},
			{"url": "https://grafana.acme.io/d/abc", "kind": "monitor"}
		]
	}`
	c, w := bugRequest(http.MethodPost, "/bugs", body)
	h.Create(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"severity":"critical"`)
	assert.Contains(t, w.Body.String(), "acme/game#42")

	// Stored with defaults.
	var stored model.Bug
	require.NoError(t, db.Where("title = ?", "iOS 充值弹窗卡死").First(&stored).Error)
	assert.Equal(t, model.BugStatusTriage, stored.Status)
	assert.Equal(t, "internal", stored.Source)
	var links []model.BugLink
	require.NoError(t, json.Unmarshal(stored.Links, &links))
	require.Len(t, links, 2)
	assert.Equal(t, model.BugLinkGithubIssue, links[0].Kind)
	assert.Equal(t, "acme/game#42", links[0].Title)

	// List filter by status/severity.
	c, w = bugRequest(http.MethodGet, "/bugs?status=triage&severity=critical", "")
	h.List(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":1`)

	// Update status + fixVersion (release board flow).
	patch := `{"status":"fixing","fixVersion":"1.5.0"}`
	c, w = bugRequest(http.MethodPut, fmt.Sprintf("/bugs/%d", stored.ID), patch)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(stored.ID)}}
	h.Update(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"fixing"`)
	assert.Contains(t, w.Body.String(), `"fixVersion":"1.5.0"`)

	// Release board filter.
	c, w = bugRequest(http.MethodGet, "/bugs?fixVersion=1.5.0", "")
	h.List(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":1`)
}

func TestBugCreate_Validation(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty title", `{"title":"  "}`, "标题"},
		{"bad severity", `{"title":"x","severity":"huge"}`, "严重度"},
		{"bad status", `{"title":"x","status":"done"}`, "状态"},
		{"bad repro", `{"title":"x","reproducibility":"never"}`, "复现率"},
		{"bad link kind", `{"title":"x","links":[{"url":"https://a.b","kind":"slack"}]}`, "kind"},
		{"bad link url", `{"title":"x","links":[{"url":"","kind":"other"}]}`, "url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := bugRequest(http.MethodPost, "/bugs", tc.body)
			h.Create(c)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tc.want)
		})
	}
}

func TestBugUpdate_NoFields(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	require.NoError(t, db.Create(&model.Bug{Title: "t", Status: model.BugStatusTriage}).Error)

	c, w := bugRequest(http.MethodPut, "/bugs/1", `{}`)
	h.Update(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBugDelete_NotFound(t *testing.T) {
	db := newBugTestDB(t)
	h := newBugHandler(db)
	c, w := bugRequest(http.MethodDelete, "/bugs/99", "")
	h.Delete(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestBugBoardOrdering(t *testing.T) {
	db := newBugTestDB(t)
	m := model.NewBugModel(db)
	for _, st := range []string{model.BugStatusReleased, model.BugStatusTriage, model.BugStatusFixing} {
		require.NoError(t, m.Create(context.Background(), &model.Bug{Title: "b-" + st, Status: st}))
	}
	items, total, err := m.List(context.Background(), model.BugQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	// triage first, fixing second, released last (board order).
	assert.Equal(t, model.BugStatusTriage, items[0].Status)
	assert.Equal(t, model.BugStatusFixing, items[1].Status)
	assert.Equal(t, model.BugStatusReleased, items[2].Status)
}
