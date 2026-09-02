package config

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var watchDBSeq uint64

func newWatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("cfg_watch_%d", atomic.AddUint64(&watchDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func TestParseNamespaces(t *testing.T) {
	ns, err := parseNamespaces("")
	require.NoError(t, err)
	assert.Equal(t, []string{"runtime"}, ns)

	ns, err = parseNamespaces("runtime, iap, gameplay")
	require.NoError(t, err)
	assert.Equal(t, []string{"runtime", "iap", "gameplay"}, ns)

	_, err = parseNamespaces("secret")
	require.Error(t, err)
}

func TestWatchSnapshotAndDiff(t *testing.T) {
	db := newWatchTestDB(t)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "runtime", Key: "login.captcha", Version: 3, Value: "true", Format: "json",
	}).Error)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "iap", Key: "shop.enabled", Version: 1, Value: "true", Format: "json",
	}).Error)

	svcCtx := &svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}
	ws := NewWatchService(svcCtx)

	snap := ws.currentVersions(context.Background(), []string{"runtime", "iap"})
	assert.Equal(t, 3, snap["runtime/login.captcha"])
	assert.Equal(t, 1, snap["iap/shop.enabled"])

	// No change → empty diff; version bump → single-entry diff.
	now := copyVersions(snap)
	assert.Empty(t, diffVersions(snap, now))
	now["runtime/login.captcha"] = 4
	changed := diffVersions(snap, now)
	assert.Equal(t, map[string]int{"runtime/login.captcha": 4}, changed)
}

func TestWatchHandler_SSEHeaders(t *testing.T) {
	db := newWatchTestDB(t)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "runtime", Key: "login.captcha", Version: 2, Value: "true", Format: "json",
	}).Error)
	svcCtx := &svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}
	h := NewWatchHandler(NewWatchService(svcCtx))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Cancel quickly: the loop heartbeats every 5s; return after snapshot.
	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/configs/watch?namespaces=runtime", nil).WithContext(ctx)

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	h.Watch(c)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "event: snapshot")
	assert.Contains(t, w.Body.String(), "login.captcha")
}

func TestPublicHandler_List(t *testing.T) {
	db := newWatchTestDB(t)
	// Latest per key wins: v1 then v2 of the same key.
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "iap", Key: "shop.enabled", Version: 1, Value: "false", Format: "json",
	}).Error)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "iap", Key: "shop.enabled", Version: 2, Value: "true", Format: "json",
	}).Error)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "runtime", Key: "other.key", Version: 1, Value: "1", Format: "json",
	}).Error)

	svcCtx := &svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}
	h := NewPublicHandler(NewPublicService(svcCtx))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/public/configs?ns=iap", nil)
	h.List(c)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"key":"shop.enabled"`)
	assert.Contains(t, body, `"value":"true"`)
	assert.Contains(t, body, `"version":2`)
	assert.NotContains(t, body, "other.key", "ns filter must exclude other namespaces")

	// Invalid namespace → 400.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/public/configs?ns=secret", nil)
	h.List(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func copyVersions(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// 轮询循环分支：无变更 → ping 心跳；版本变更 → changed 事件；取消 → 返回。
func TestWatchHandler_Loop_PingChangedDone(t *testing.T) {
	db := newWatchTestDB(t)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "runtime", Key: "k1", Version: 1, Value: "v", Format: "json",
	}).Error)
	svcCtx := &svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}
	h := NewWatchHandler(NewWatchService(svcCtx))
	h.interval = 10 * time.Millisecond

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/configs/watch?namespaces=runtime", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Watch(c)
	}()

	// 等几轮 tick：前几轮无变更 → ping
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(w.Body.String(), ": ping") {
		time.Sleep(5 * time.Millisecond)
	}

	// 版本 bump → 下一轮 tick 发 changed
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "runtime", Key: "k1", Version: 2, Value: "v2", Format: "json",
	}).Error)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(w.Body.String(), "event: changed") {
		time.Sleep(5 * time.Millisecond)
	}
	assert.Contains(t, w.Body.String(), "event: changed", w.Body.String())
	assert.Contains(t, w.Body.String(), `"runtime/k1":2`)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel 后 Watch 未返回")
	}
}

// currentVersions 查询失败 → 告警并返回空表（不 panic）。
func TestWatchService_CurrentVersions_DBError(t *testing.T) {
	db := newWatchTestDB(t)
	require.NoError(t, db.Migrator().DropTable("config_versions"))
	ws := NewWatchService(&svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)})
	out := ws.currentVersions(context.Background(), []string{"runtime"})
	assert.Empty(t, out)
}

// parseNamespaces 重复项去重。
func TestParseNamespaces_Dedup(t *testing.T) {
	ns, err := parseNamespaces("runtime,runtime")
	require.NoError(t, err)
	assert.Equal(t, []string{"runtime"}, ns)
}
