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

// failingBindValidatorV12 injects a failure at gin's pluggable validator
// extension point, making ShouldBindQuery return an error at the validate
// stage after mapForm has succeeded. Under the default validator,
// ResourceListRequest holds only string form fields and binding always
// succeeds (see deadbranch_extra_test.go), so the binding error branch of
// handler.go List is reachable only through this extension point.
type failingBindValidatorV12 struct{}

func (failingBindValidatorV12) ValidateStruct(interface{}) error {
	return errors.New("forced bind validation failure")
}

func (failingBindValidatorV12) Engine() any { return nil }

// TestHandlerListBindErrorV12 covers the ShouldBindQuery error branch of
// handler.go List: a binding failure should return directly through
// response.Error without entering service.List.
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
