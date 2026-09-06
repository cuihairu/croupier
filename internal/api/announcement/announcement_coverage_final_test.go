// 覆盖目标（coverage final）：
//  1. handler.Create 的 ShouldBindJSON 失败分支（Validator 注入）。
//  2. handler.Active 的 service 错误分支（认证通过后 announcements 表损坏）。
//  3. handler.Dismiss 的 LoadCurrentAdmin 失败分支（无认证上下文）。
//  4. service.Update 的 Updates 失败（update callback 注错）。
//  5. service.Update 的回读 First 失败（query callback 按次注错）。
package announcement

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type annFailValidator struct{}

func (annFailValidator) ValidateStruct(any) error { return errors.New("injected validate failure") }
func (annFailValidator) Engine() any              { return nil }

func TestAnnouncementHandler_Create_BindValidatorFailure(t *testing.T) {
	orig := binding.Validator
	binding.Validator = annFailValidator{}
	t.Cleanup(func() { binding.Validator = orig })

	_, r := newHandlerFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/announcements", strings.NewReader(`{"title":"t"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// 注入的非 ValidationErrors 错误经 response.Error 走默认 500 映射
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAnnouncementHandler_Active_ServiceError_Authenticated(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}, &model.Admin{}, &model.Role{}, &model.AdminRole{}))
	svcCtx := &svc.ServiceContext{DB: db, AdminModel: model.NewAdminModel(db)}
	require.NoError(t, svcCtx.AdminModel.Create(context.Background(), &model.Admin{Username: "alice", Nickname: "A"}, "x"))
	h := NewHandler(NewService(svcCtx))

	// 认证通过 + announcements 表缺失 → ActiveForUser 查询失败 → 500。
	require.NoError(t, db.Migrator().DropTable("announcements"))
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/announcements/active", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "username", "alice"))
	h.Active(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAnnouncementHandler_Dismiss_LoadCurrentAdminFailure(t *testing.T) {
	_, r := newHandlerFixture(t)

	// 合法 ID + 无认证上下文 → LoadCurrentAdmin 失败（CurrentUsername 的
	// 原生 error 不带 401 语义）→ response.Error 默认 500。
	req := httptest.NewRequest(http.MethodPost, "/announcements/1/dismiss", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAnnouncementService_Update_Failures(t *testing.T) {
	ctx := context.Background()

	// Updates 落库失败。
	db1, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db1.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}))
	s1 := NewService(&svc.ServiceContext{DB: db1})
	created1, err := s1.Create(ctx, &CreateRequest{Title: "t1", ContentMd: "c", Audience: "all"})
	require.NoError(t, err)
	require.NoError(t, db1.Callback().Update().Before("gorm:update").
		Register("test:ann_fail_update", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("update boom"))
		}))
	_, err = s1.Update(ctx, uint(created1.ID), &UpdateRequest{Title: strPtr("x")})
	require.ErrorContains(t, err, "update boom")

	// 回读 First 失败：Update 流程 SELECT 依次 First → (Updates) → 回读 First。
	db2, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db2.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}))
	s2 := NewService(&svc.ServiceContext{DB: db2})
	created2, err := s2.Create(ctx, &CreateRequest{Title: "t2", ContentMd: "c", Audience: "all"})
	require.NoError(t, err)
	var selects int32
	require.NoError(t, db2.Callback().Query().Before("gorm.query").
		Register("test:ann_fail_reload", func(tx *gorm.DB) {
			if atomic.AddInt32(&selects, 1) == 2 {
				_ = tx.AddError(errors.New("reload boom"))
			}
		}))
	_, err = s2.Update(ctx, uint(created2.ID), &UpdateRequest{Title: strPtr("y")})
	require.ErrorContains(t, err, "reload boom")
}
