// 回归测试：GET /domain-info 契约是 query 风格（docs/operations/certificates.md
// `?domain=xxx`，路由注册无 :domain 路径参数）。此前 handler 误用
// ShouldBindUri 且 DTO 只有 form tag，Domain 恒空 → 端点恒 400（线上不可用）。
package certificate

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerGetDomainInfoQueryContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("domain in query returns stored certificate", func(t *testing.T) {
		service, db := setupTestService(t)
		expires := time.Now().Add(90 * 24 * time.Hour)
		require.NoError(t, db.Create(&model.Certificate{
			Domain: "example.com", Port: 443, Issuer: "Test CA",
			ExpiresAt: expires, Status: model.CertificateStatus(expires),
		}).Error)
		h := NewHandler(service)

		r := gin.New()
		r.GET("/certificates/domain-info", h.GetDomainInfo)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/certificates/domain-info?domain=example.com", nil)
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "example.com")
	})

	t.Run("missing domain is rejected", func(t *testing.T) {
		service, _ := setupTestService(t)
		h := NewHandler(service)

		r := gin.New()
		r.GET("/certificates/domain-info", h.GetDomainInfo)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/certificates/domain-info", nil)
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unknown domain maps to 404 not 500", func(t *testing.T) {
		// FindByDomain 此前用 fmt.Errorf 断链，"证书不存在"落 500 兜底；
		// wrap ErrRecordNotFound 后 response.Error 应映射 404。
		service, _ := setupTestService(t)
		h := NewHandler(service)

		r := gin.New()
		r.GET("/certificates/domain-info", h.GetDomainInfo)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/certificates/domain-info?domain=absent.example.com", nil)
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), "not_found")
	})
}
