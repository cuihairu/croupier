// Fills in the remaining coverable branches of the feedback package (group C):
// the uninitialized FeedbackModel branch of service.Update.
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
