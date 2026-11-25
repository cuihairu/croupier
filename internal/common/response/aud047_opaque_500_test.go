package response

// AUD-047 regression: unknown errors uniformly return an opaque 500.
//
// Root cause: the Error fallback branch returned the err.Error() text
// verbatim to the client — GORM/driver errors would leak SQL fragments,
// table names, dialect error codes and other internal details. Fix: the
// fallback returns only a stable message plus correlationId (structured
// details + a message hint); the original text goes to server logs via
// slog, traceable by correlation ID.

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aud047Fire issues one request against the Error fallback with the given error.
func aud047Fire(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/things", func(c *gin.Context) {
		Error(c, err)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/things", nil)
	router.ServeHTTP(w, req)
	return w
}

func TestAud047_UnknownErrorOpaque(t *testing.T) {
	// Typical leak source: the raw driver error contains table names and
	// dialect details
	raw := `pq: relation "game_demo_prod.players" does not exist, seq scan on players_pkey`
	w := aud047Fire(t, fmt.Errorf("%s", raw))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"error":"internal_error"`)
	// No fragment of the raw text may appear ("relation" cannot be used
	// for the assertion — correlationId itself contains that substring,
	// so the leak criteria land on table names and dialect prefixes)
	assert.NotContains(t, body, "game_demo_prod")
	assert.NotContains(t, body, "players_pkey")
	assert.NotContains(t, body, "pq:")
	// The correlation ID lives in the structured details field
	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Details struct {
			CorrelationID string `json:"correlationId"`
		} `json:"details"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &payload))
	require.NotEmpty(t, payload.Details.CorrelationID)
	decoded, derr := hex.DecodeString(payload.Details.CorrelationID)
	require.NoError(t, derr, "correlationId 必须是合法 hex")
	assert.Len(t, decoded, 8)
	assert.Contains(t, payload.Message, payload.Details.CorrelationID,
		"message 提示带追踪号，运维可凭响应回查日志")
}

func TestAud047_CorrelationIDUniquePerError(t *testing.T) {
	w1 := aud047Fire(t, fmt.Errorf("boom one"))
	w2 := aud047Fire(t, fmt.Errorf("boom two"))
	require.Equal(t, http.StatusInternalServerError, w1.Code)
	require.Equal(t, http.StatusInternalServerError, w2.Code)
	var p1, p2 struct {
		Details struct {
			CorrelationID string `json:"correlationId"`
		} `json:"details"`
	}
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &p1))
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &p2))
	assert.NotEqual(t, p1.Details.CorrelationID, p2.Details.CorrelationID,
		"两次错误各带独立追踪号")
}
