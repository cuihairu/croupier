// Coverage target: the query binding failure branch of Handler.DeleteObject
// (DeleteObjectRequest holds only string fields, so mapForm cannot be made to
// fail with an out-of-range value the way SignedUrl/ListObjects can; instead a
// failing Validator is injected to force validate to error -- the same approach
// as the coverage_final tests in the node/console/config packages).
package storage

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

// storageFailValidator makes the validate step of any Bind return an error.
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
