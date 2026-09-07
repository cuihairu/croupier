package resource

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
)

// failingBindValidatorV12 在 gin 的可插拔校验器扩展点上注入失败，
// 使 ShouldBindQuery 在 mapForm 成功后于 validate 阶段返回错误。
// 默认 validator 下 ResourceListRequest 仅含 string form 字段、绑定恒成功
// （见 deadbranch_extra_test.go），因此 handler.go List 的绑定错误分支
// 只能通过该扩展点触达。
type failingBindValidatorV12 struct{}

func (failingBindValidatorV12) ValidateStruct(interface{}) error {
	return errors.New("forced bind validation failure")
}

func (failingBindValidatorV12) Engine() any { return nil }

// TestHandlerListBindErrorV12 覆盖 handler.go List 的 ShouldBindQuery 错误分支：
// 绑定失败时应直接经 response.Error 返回，不再进入 service.List。
func TestHandlerListBindErrorV12(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orig := binding.Validator
	binding.Validator = failingBindValidatorV12{}
	t.Cleanup(func() { binding.Validator = orig })

	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	h := NewHandler(NewService(svcCtx))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources?category=player", nil)
	c.Request = req.WithContext(ctx)

	h.List(c)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")
}
