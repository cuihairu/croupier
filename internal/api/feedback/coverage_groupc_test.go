// 补齐 feedback 包剩余可覆盖分支（group C）：
// service.Update 的 FeedbackModel 未初始化分支。
package feedback

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeedbackUpdate_ModelNotInitialized(t *testing.T) {
	s := NewService(&svc.ServiceContext{})

	_, err := s.Update(context.Background(), &FeedbackUpdateRequest{Status: "open"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "反馈模型未初始化")
}
