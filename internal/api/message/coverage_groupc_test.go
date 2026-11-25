// Closing out the remaining coverable branches of package message (group C):
// the ShouldBindQuery failure branch of handler.UnreadCount. The request DTO is
// an empty struct, so form mapping cannot fail and a forced validation failure
// is needed via Validator injection (same precedent as package console).
package message

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
)

type msgFailValidator struct{}

func (msgFailValidator) ValidateStruct(any) error { return errors.New("forced query bind failure") }
func (msgFailValidator) Engine() any              { return nil }

func TestHandlerUnreadCount_QueryBindFailure(t *testing.T) {
	orig := binding.Validator
	binding.Validator = msgFailValidator{}
	t.Cleanup(func() { binding.Validator = orig })

	handler := newMessageHandler(newMessageTestDB(t))
	ctx, rec := newMessageRequest(http.MethodGet, "/api/v1/messages/unread-count", "")
	handler.UnreadCount(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assertMessageErrorShape(t, rec)
}
