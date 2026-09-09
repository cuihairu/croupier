package feedback

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Delete 成功路径：先创建反馈再删除，handler 应回 200 与操作成功消息。
func TestHandler_Delete_Success(t *testing.T) {
	db := newFeedbackTestDB(t)
	handler := newFeedbackHandler(db)

	createCtx, createRec := newFeedbackRequest(http.MethodPost, "/api/v1/feedback",
		`{"contact":"player1@example.com","content":"delete me","category":"bug","rating":2,"gameId":"demo","env":"prod"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code, createRec.Body.String())

	var created FeedbackCreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	require.NotZero(t, created.Id)
	idStr := strconv.FormatInt(created.Id, 10)

	delCtx, delRec := newFeedbackRequest(http.MethodDelete, "/api/v1/feedback/"+idStr, "")
	delCtx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.Delete(delCtx)

	require.Equal(t, http.StatusOK, delRec.Code, delRec.Body.String())
	assert.Contains(t, delRec.Body.String(), "操作成功")

	// 删除后再查应 404/错误，不应残留
	listCtx, listRec := newFeedbackRequest(http.MethodGet, "/api/v1/feedback?page=1&pageSize=10", "")
	handler.List(listCtx)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listResp FeedbackListResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	assert.Empty(t, listResp.Items)
}
