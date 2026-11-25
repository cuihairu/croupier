// Coverage target (group E): the query binding error branch of certificate
// handler.GetDomainInfo (handler.go:144-147).
//
// CertificateDomainInfoRequest has only string fields with form tags (no validation
// tags), so query binding always succeeds for plain strings; the error branch is only
// reachable by injecting a failure at the validate stage via gin's pluggable validator
// extension point.
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

// failingBindValidatorGroupE makes ShouldBindQuery return an error at the validate
// stage after mapForm succeeds.
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

	// Mount in the production route shape (query style, no :domain path param).
	r := gin.New()
	r.GET("/certificates/domain-info", h.GetDomainInfo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/certificates/domain-info?domain=example.com", nil)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")
}
