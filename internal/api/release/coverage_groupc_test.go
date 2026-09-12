// 补齐 release 包剩余可覆盖分支（group C）：
// handler.UploadArtifact 的 ShouldBindUri 校验失败分支。uri DTO 仅含单个
// string 字段，mapURI 无法失败，须以 Validator 注入强制校验失败。
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
