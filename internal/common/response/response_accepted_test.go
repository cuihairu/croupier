package response

// response.go 补测：Accepted 构造（202 异步受理响应）。
//
// 无诚实跳过：单分支构造。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccepted(t *testing.T) {
	router := gin.New()
	router.GET("/ok", func(c *gin.Context) {
		Accepted(c, gin.H{"task": "queued"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "queued", body["task"])
}
