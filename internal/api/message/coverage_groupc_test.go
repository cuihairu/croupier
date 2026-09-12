// 补齐 message 包剩余可覆盖分支（group C）：
// handler.UnreadCount 的 ShouldBindQuery 失败分支。请求 DTO 是空结构体，
// form 映射无法失败，须以 Validator 注入强制校验失败（同 console 包先例）。
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
