// 覆盖目标（组 E）：alert handler.RulesList 的 ShouldBindQuery 错误分支
// （handler.go:78-81）。
//
// RulesListRequest 仅含两个无校验 tag 的 string form 字段，绑定恒成功
// （见 resource/deadbranch_extra_test.go 的同类结论），错误分支只能通过
// gin 的可插拔校验器扩展点在 validate 阶段注入失败触达。
package alert

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
)

// failingBindValidatorGroupE 使 ShouldBindQuery 在 mapForm 成功后于
// validate 阶段返回错误。
type failingBindValidatorGroupE struct{}

func (failingBindValidatorGroupE) ValidateStruct(interface{}) error {
	return errors.New("forced bind validation failure")
}

func (failingBindValidatorGroupE) Engine() any { return nil }

func TestHandlerRulesListBindErrorGroupE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orig := binding.Validator
	binding.Validator = failingBindValidatorGroupE{}
	t.Cleanup(func() { binding.Validator = orig })

	_, s, _ := newAlertRulesEnv(t)
	h := NewHandler(s)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/alerts/rules?metric=cpu.usagePercent", nil)

	h.RulesList(c)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")
}
