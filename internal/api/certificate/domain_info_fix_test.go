// Regression test: the GET /domain-info contract is query-style
// (docs/operations/certificates.md `?domain=xxx`; the route registration has no :domain
// path param). Previously the handler misused ShouldBindUri and the DTO only had form
// tags, so Domain stayed empty -> the endpoint always returned 400 (unusable in
// production).
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
		// FindByDomain previously used fmt.Errorf and broke the chain, so "certificate
		// not found" fell into the 500 fallback; after wrapping ErrRecordNotFound,
		// response.Error should map it to 404.
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
