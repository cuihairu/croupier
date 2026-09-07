// 覆盖目标：Handler.List 的 query 绑定失败分支（handler.go List 的
// ShouldBindQuery err 分支）。TermsListRequest 仅含无校验 tag 的 string
// 字段，mapForm 无法自然失败，因此注入失败 Validator 在 validate 阶段
// 强制报错（与 node/console/config 包的 coverage_final 同法）。
package terms

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
)

// termsFailValidator 让任意 Bind 的 validate 步骤返回错误。
type termsFailValidator struct{}

func (termsFailValidator) ValidateStruct(any) error { return errors.New("forced bind failure") }
func (termsFailValidator) Engine() any              { return nil }

func withTermsFailingValidator(t *testing.T) {
	t.Helper()
	orig := binding.Validator
	binding.Validator = termsFailValidator{}
	t.Cleanup(func() { binding.Validator = orig })
}

func TestHandler_List_BindValidatorFailure(t *testing.T) {
	withTermsFailingValidator(t)
	handler := newTermsHandler(newTermsTestDB(t))

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/terms?domain=resource", nil)

	handler.List(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertTermsErrorShape(t, rec)
}
