// Coverage targets (coverage final):
//  1. The ShouldBindJSON failure branch in handler.Create (injected Validator).
//  2. The service error branch in handler.Active (announcements table missing
//     after authentication passes).
//  3. The LoadCurrentAdmin failure branch in handler.Dismiss (no auth context).
//  4. The Updates failure in service.Update (injected update callback).
//  5. The read-back First failure in service.Update (per-call query callback).
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
	// The injected non-ValidationErrors error goes through response.Error and
	// takes the default 500 mapping.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAnnouncementHandler_Active_ServiceError_Authenticated(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}, &model.AnnouncementGame{}, &model.Admin{}, &model.Role{}, &model.AdminRole{}))
	svcCtx := &svc.ServiceContext{DB: db, AdminModel: model.NewAdminModel(db)}
	require.NoError(t, svcCtx.AdminModel.Create(context.Background(), &model.Admin{Username: "alice", Nickname: "A"}, "x"))
	h := NewHandler(NewService(svcCtx))

	// Authenticated + announcements table missing -> the ActiveForUser query
	// fails -> 500.
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

	// Well-formed ID + no auth context -> LoadCurrentAdmin fails (the raw
	// error from CurrentUsername carries no 401 semantics) -> response.Error
	// defaults to 500.
	req := httptest.NewRequest(http.MethodPost, "/announcements/1/dismiss", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAnnouncementService_Update_Failures(t *testing.T) {
	ctx := context.Background()

	// The Updates write to the database fails.
	db1, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db1.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}, &model.AnnouncementGame{}))
	s1 := NewService(&svc.ServiceContext{DB: db1})
	created1, err := s1.Create(ctx, &CreateRequest{Title: "t1", ContentMd: "c", Audience: "all"})
	require.NoError(t, err)
	require.NoError(t, db1.Callback().Update().Before("gorm:update").
		Register("test:ann_fail_update", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("update boom"))
		}))
	_, err = s1.Update(ctx, uint(created1.ID), &UpdateRequest{Title: strPtr("x")})
	require.ErrorContains(t, err, "update boom")

	// The read-back First fails: the Update flow SELECTs as First ->
	// (Updates) -> read-back First.
	db2, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db2.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}, &model.AnnouncementGame{}))
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
