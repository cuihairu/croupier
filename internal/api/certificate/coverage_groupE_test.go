// 覆盖目标（组 E）：certificate handler.GetDomainInfo 的 query 绑定错误
// 分支（handler.go:144-147）。
//
// CertificateDomainInfoRequest 只有 form tag 的 string 字段（无校验
// tag），query 绑定对纯 string 恒成功，错误分支只能通过 gin 的可
// 插拔校验器扩展点在 validate 阶段注入失败触达。
package certificate

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
)

// failingBindValidatorGroupE 使 ShouldBindQuery 在 mapForm 成功后于 validate
// 阶段返回错误。
type failingBindValidatorGroupE struct{}

func (failingBindValidatorGroupE) ValidateStruct(interface{}) error {
	return errors.New("forced bind validation failure")
}

func (failingBindValidatorGroupE) Engine() any { return nil }

func TestHandlerGetDomainInfoBindErrorGroupE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orig := binding.Validator
	binding.Validator = failingBindValidatorGroupE{}
	t.Cleanup(func() { binding.Validator = orig })

	service, _ := setupTestService(t)
	h := NewHandler(service)

	// 挂载为线上路由形态（query 风格，无 :domain 路径参数）。
	r := gin.New()
	r.GET("/certificates/domain-info", h.GetDomainInfo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/certificates/domain-info?domain=example.com", nil)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")
}
