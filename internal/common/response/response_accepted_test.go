package response

// Supplementary tests for response.go: Accepted construction (202 async
// acceptance response).
//
// No honest-skip case: single-branch constructor.

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
