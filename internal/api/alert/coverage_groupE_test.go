// Coverage target (group E): the ShouldBindQuery error branch of alert handler.RulesList
// (handler.go:78-81).
//
// RulesListRequest only carries two string form fields without validation tags, so
// binding always succeeds (same conclusion as resource/deadbranch_extra_test.go); the
// error branch can only be reached by injecting a failure at the validate stage via
// gin's pluggable validator extension point.
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

// failingBindValidatorGroupE makes ShouldBindQuery return an error at the validate
// stage after mapForm has succeeded.
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
