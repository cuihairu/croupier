package message

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 群发夹具：boss=admin 角色，alice=ops 角色；两张表都在内存库中。
func newBroadcastAuthFixture(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.AdminRole{}))
	adminModel := model.NewAdminModel(db)
	require.NoError(t, adminModel.Create(context.Background(), &model.Admin{Username: "boss", Nickname: "Boss"}, "x"))
	require.NoError(t, adminModel.Create(context.Background(), &model.Admin{Username: "alice", Nickname: "Alice"}, "x"))
	boss, err := adminModel.FindByUsername(context.Background(), "boss")
	require.NoError(t, err)
	alice, err := adminModel.FindByUsername(context.Background(), "alice")
	require.NoError(t, err)
	adminRole := &model.Role{Name: "admin"}
	opsRole := &model.Role{Name: "ops"}
	require.NoError(t, db.Create(adminRole).Error)
	require.NoError(t, db.Create(opsRole).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), boss.ID, adminRole.ID))
	require.NoError(t, adminModel.AssignRole(context.Background(), alice.ID, opsRole.ID))

	svcCtx := &svc.ServiceContext{MessageModel: model.NewMessageModel(db), AdminModel: adminModel}
	h := NewHandler(NewService(svcCtx), config.SSEConfig{})
	r := gin.New()
	r.POST("/messages/broadcast", h.Broadcast)
	r.GET("/messages/unread-count", h.UnreadCount)
	r.GET("/messages", h.List)
	return h, r
}

func broadcastReq(r *gin.Engine, method, path, body, username string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, path, reader)
	if username != "" {
		req = req.WithContext(context.WithValue(req.Context(), "username", username))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_Broadcast_Auth(t *testing.T) {
	_, r := newBroadcastAuthFixture(t)

	// 未认证 → 403
	w := broadcastReq(r, http.MethodPost, "/messages/broadcast", `{}`, "")
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// 非 admin 角色 → 403
	w = broadcastReq(r, http.MethodPost, "/messages/broadcast", `{"type":"system","title":"t","content":"c","audience":"users","usernames":["alice"]}`, "alice")
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestHandler_Broadcast_Admin(t *testing.T) {
	_, r := newBroadcastAuthFixture(t)

	// admin 指定用户群发 → 200 且计数正确
	body := `{"type":"system","title":"停服通知","content":"今晚维护","audience":"users","usernames":["alice","boss"]}`
	w := broadcastReq(r, http.MethodPost, "/messages/broadcast", body, "boss")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"sent":2`)

	// audience=all → 全员（boss+alice）
	w = broadcastReq(r, http.MethodPost, "/messages/broadcast", `{"type":"system","title":"全员","content":"c","audience":"all"}`, "boss")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"sent":2`)

	// audience=role → 按角色
	w = broadcastReq(r, http.MethodPost, "/messages/broadcast", `{"type":"system","title":"按角色","content":"c","audience":"role","role":"ops"}`, "boss")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"sent":1`)

	// 非法 body → 400
	w = broadcastReq(r, http.MethodPost, "/messages/broadcast", `{bad`, "boss")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 标题为空 / 内容为空 / 类型为空 → 错误
	for _, b := range []string{
		`{"type":"system","title":"","content":"c","audience":"users","usernames":["alice"]}`,
		`{"type":"system","title":"t","content":"","audience":"users","usernames":["alice"]}`,
		`{"type":"","title":"t","content":"c","audience":"users","usernames":["alice"]}`,
		`{"type":"system","title":"t","content":"c","audience":"users"}`,
		`{"type":"system","title":"t","content":"c","audience":"role"}`,
	} {
		w = broadcastReq(r, http.MethodPost, "/messages/broadcast", b, "boss")
		assert.NotEqual(t, http.StatusOK, w.Code, b)
	}
}

func TestHandler_UnreadCount_StatusFilter(t *testing.T) {
	_, r := newBroadcastAuthFixture(t)

	// 先群发一条给 alice，制造未读
	w := broadcastReq(r, http.MethodPost, "/messages/broadcast", `{"type":"system","title":"t","content":"c","audience":"users","usernames":["alice"]}`, "boss")
	require.Equal(t, http.StatusOK, w.Code)

	// 无 status → 全部未读计数（含全部状态分支）
	w = broadcastReq(r, http.MethodGet, "/messages/unread-count", "", "alice")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// status=unread / bogus / 大小写 → parseMessageStatusFilter 各分支
	for _, q := range []string{"?status=unread", "?status=READ", "?status=bogus", "?status="} {
		w = broadcastReq(r, http.MethodGet, "/messages/unread-count"+q, "", "alice")
		assert.Equal(t, http.StatusOK, w.Code, q)
	}
}

func TestHandler_List_Branches(t *testing.T) {
	_, r := newBroadcastAuthFixture(t)

	// 正常 List（带 status/大小写/非法值 → parseMessageStatusFilter 分支）
	for _, q := range []string{"", "?status=unread", "?status=READ", "?status=bogus", "?page=1&pageSize=5"} {
		w := broadcastReq(r, http.MethodGet, "/messages"+q, "", "alice")
		assert.Equal(t, http.StatusOK, w.Code, q)
	}

	// page=abc → Bug4 修复后 400（bind 错误分支）
	w := broadcastReq(r, http.MethodGet, "/messages?page=abc", "", "alice")
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// unread-count 无 int 字段，任意 query 都合法 → 200
	w = broadcastReq(r, http.MethodGet, "/messages/unread-count?page=abc", "", "alice")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// DB 故障注入 → List/UnreadCount 的 service 错误分支
func TestHandler_DBError_Branches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.AdminRole{}))
	adminModel := model.NewAdminModel(db)
	require.NoError(t, adminModel.Create(context.Background(), &model.Admin{Username: "alice"}, "x"))
	svcCtx := &svc.ServiceContext{MessageModel: model.NewMessageModel(db), AdminModel: adminModel}
	h := NewHandler(NewService(svcCtx), config.SSEConfig{})
	r := gin.New()
	r.GET("/messages", h.List)
	r.GET("/messages/unread-count", h.UnreadCount)
	require.NoError(t, db.Migrator().DropTable("messages"))

	w := broadcastReq(r, http.MethodGet, "/messages", "", "alice")
	assert.NotEqual(t, http.StatusOK, w.Code, w.Body.String())

	w = broadcastReq(r, http.MethodGet, "/messages/unread-count", "", "alice")
	assert.NotEqual(t, http.StatusOK, w.Code, w.Body.String())
}
