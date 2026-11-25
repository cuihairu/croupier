// Coverage target: the query-binding failure branch of Handler.List (the
// ShouldBindQuery err branch in List, handler.go). TermsListRequest only has
// string fields without validation tags, so mapForm cannot fail naturally;
// a failing Validator is injected to force an error at the validate stage
// (same approach as the coverage_final tests in the node/console/config packages).
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

// termsFailValidator makes the validate step of any Bind return an error.
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
