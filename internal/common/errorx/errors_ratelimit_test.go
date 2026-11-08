package errorx

// errors.go 补测：NewTooManyRequests 构造（429 + 固定 rate_limited 码）。
//
// 无诚实跳过：纯构造函数。

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTooManyRequests(t *testing.T) {
	err := NewTooManyRequests("too many attempts")
	require.Equal(t, http.StatusTooManyRequests, err.Code)
	require.Equal(t, "rate_limited", err.StableCode)
	require.Equal(t, "too many attempts", err.Message)
	require.Nil(t, err.Details)
}
