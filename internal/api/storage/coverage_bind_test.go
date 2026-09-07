// 覆盖目标：Handler.DeleteObject 的 query 绑定失败分支
// （DeleteObjectRequest 仅含 string 字段，无法像 SignedUrl/ListObjects 那样
// 用非法数值触发 mapForm 失败，因此注入失败 Validator 强制 validate 报错，
// 与 node/console/config 包的 coverage_final 同法）。
package storage

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

// storageFailValidator 让任意 Bind 的 validate 步骤返回错误。
type storageFailValidator struct{}

func (storageFailValidator) ValidateStruct(any) error { return errors.New("forced bind failure") }
func (storageFailValidator) Engine() any              { return nil }

func withStorageFailingValidator(t *testing.T) {
	t.Helper()
	orig := binding.Validator
	binding.Validator = storageFailValidator{}
	t.Cleanup(func() { binding.Validator = orig })
}

func TestCoverageHandler_DeleteObject_BindValidatorFailure(t *testing.T) {
	withStorageFailingValidator(t)

	router := extraRouter(setupHandler(t))
	rec := doExtraReq(t, router, httptest.NewRequest(http.MethodDelete, "/storage/objects?path=a.txt", nil))
	assertStatus(t, rec, http.StatusInternalServerError)
	assertErrorShape(t, rec)
}
