package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AppError.Message 为空时，Error 用 err.Error() 兜底，避免透出空 message。
func TestErrorAppEmptyMessageFallsBackToErrorTextG(t *testing.T) {
	gin.SetMode(gin.TestMode)
	appErr := &errors.AppError{
		Code:           errors.ErrCodeInternal,
		HTTPStatusCode: http.StatusServiceUnavailable,
		Operation:      "op-g",
		Details:        "detail-g",
	}

	router := gin.New()
	router.GET("/e", func(c *gin.Context) { Error(c, appErr) })
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/e", nil)
	require.NoError(t, err)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body struct {
		Error   string            `json:"error"`
		Message string            `json:"message"`
		Details map[string]string `json:"details"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "internal_error", body.Error)
	assert.Contains(t, body.Message, string(errors.ErrCodeInternal))
	assert.Equal(t, "op-g", body.Details["operation"])
	assert.Equal(t, "detail-g", body.Details["detail"])
}
