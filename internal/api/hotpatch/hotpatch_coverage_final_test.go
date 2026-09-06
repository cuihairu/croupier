// 覆盖目标（coverage final）：
//  1. handler.UploadPackage 的 ShouldBindUri 失败分支（Validator 注入）。
//  2. service.UploadPackage 的 HotpatchModel.Update 失败（update callback 注错）。
//  3. service.UploadPackage 的回读 FindOne 失败（query callback 按次注错）。
//  4. service.Transition 的 rollback 分支。
package hotpatch

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type hpFailValidator struct{}

func (hpFailValidator) ValidateStruct(any) error { return errors.New("injected validate failure") }
func (hpFailValidator) Engine() any              { return nil }

func TestHotpatchHandler_UploadPackage_BindUriFailure(t *testing.T) {
	orig := binding.Validator
	binding.Validator = hpFailValidator{}
	t.Cleanup(func() { binding.Validator = orig })

	gin.SetMode(gin.TestMode)
	s := newFixture(t)
	h := NewHandler(s)
	r := gin.New()
	r.POST("/hotpatches/:id/package", h.UploadPackage)

	var body strings.Builder
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "patch.bin")
	require.NoError(t, err)
	_, err = fw.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/hotpatches/1/package", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHotpatchService_UploadPackage_UpdateFailure(t *testing.T) {
	ctx := context.Background()
	svcSrv, db := newFixtureWithDB(t)
	hp := seedDraft(t, svcSrv)

	require.NoError(t, db.Callback().Update().Before("gorm:update").
		Register("test:hp_fail_update", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("update boom"))
		}))

	_, err := svcSrv.UploadPackage(ctx, &UploadRequest{
		ID: fmt.Sprint(hp.Id), Data: strings.NewReader("payload"), Size: 7,
	})
	require.ErrorContains(t, err, "update boom")
}

func TestHotpatchService_UploadPackage_ReloadFailure(t *testing.T) {
	ctx := context.Background()
	svcSrv, db := newFixtureWithDB(t)
	hp := seedDraft(t, svcSrv)

	// UploadPackage 内 SELECT 依次为：FindOne(draft 校验) → 回读 FindOne。
	// 令第 2 次 SELECT 失败：Update 落库成功后回读报错。
	var selects int32
	require.NoError(t, db.Callback().Query().Before("gorm.query").
		Register("test:hp_fail_second_select", func(tx *gorm.DB) {
			if atomic.AddInt32(&selects, 1) == 2 {
				_ = tx.AddError(errors.New("reload boom"))
			}
		}))

	_, err := svcSrv.UploadPackage(ctx, &UploadRequest{
		ID: fmt.Sprint(hp.Id), Data: strings.NewReader("payload"), Size: 7,
	})
	require.ErrorContains(t, err, "reload boom")
}

func TestHotpatchService_Transition_Rollback(t *testing.T) {
	ctx := context.Background()
	svcSrv, _ := newFixtureWithDB(t)
	hp := seedDraft(t, svcSrv)

	// 上传补丁包（状态机要求 approve 前完成上传）。
	_, err := svcSrv.UploadPackage(ctx, &UploadRequest{
		ID: fmt.Sprint(hp.Id), Data: strings.NewReader("payload"), Size: 7,
		ContentType: "application/octet-stream",
	})
	require.NoError(t, err)

	// draft → approve → roll → applied → rollback 状态机推进到 rolled_back。
	_, err = svcSrv.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "approve"})
	require.NoError(t, err)
	_, err = svcSrv.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "roll"})
	require.NoError(t, err)
	_, err = svcSrv.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "applied"})
	require.NoError(t, err)
	final, err := svcSrv.Transition(ctx, &TransitionRequest{ID: fmt.Sprint(hp.Id), Action: "rollback"})
	require.NoError(t, err)
	assert.Equal(t, model.HotpatchStatusRolledBack, final.Status)
}
