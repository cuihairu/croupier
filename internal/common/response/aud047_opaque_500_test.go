package response

// AUD-047 回归：未知错误统一 opaque 500。
//
// 根因：Error 兜底分支把 err.Error() 原文回传客户端——GORM/驱动错误会
// 泄漏 SQL 片段、表名、方言错误码等内部细节。修复：兜底只回稳定文案 +
// correlationId（details 结构化 + message 提示），原文经 slog 进服务端
// 日志，凭追踪号回查。

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

// aud047Fire 以指定错误打一次走 Error 兜底的请求。
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
	// 典型泄漏源：驱动错误原文含表名/方言细节
	raw := `pq: relation "game_demo_prod.players" does not exist, seq scan on players_pkey`
	w := aud047Fire(t, fmt.Errorf("%s", raw))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"error":"internal_error"`)
	// 原文任何片段不得出现（"relation" 无法用于断言——correlationId 自身
	// 含该子串，泄漏判据落在表名/方言前缀上）
	assert.NotContains(t, body, "game_demo_prod")
	assert.NotContains(t, body, "players_pkey")
	assert.NotContains(t, body, "pq:")
	// 追踪号在 details 结构化字段
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
