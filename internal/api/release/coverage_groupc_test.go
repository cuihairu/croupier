// Fills in the remaining coverable branches of the release package
// (group C): the ShouldBindUri validation failure branch of
// handler.UploadArtifact. The uri DTO holds a single string field, so mapURI
// cannot fail; a Validator injection is needed to force the validation
// failure.
package release

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
)

type relFailValidator struct{}

func (relFailValidator) ValidateStruct(any) error { return errors.New("forced uri bind failure") }
func (relFailValidator) Engine() any              { return nil }

func TestHandlerUploadArtifact_UriValidationFailure(t *testing.T) {
	orig := binding.Validator
	binding.Validator = relFailValidator{}
	t.Cleanup(func() { binding.Validator = orig })

	f := newFixture(t)
	h := NewHandler(f.svc)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/releases/1/artifact", nil)

	h.UploadArtifact(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}
