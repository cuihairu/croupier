package errorx

// Supplementary tests for errors.go: NewTooManyRequests construction (429 +
// fixed rate_limited code).
//
// No honest-skip case: this is a pure constructor.

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
