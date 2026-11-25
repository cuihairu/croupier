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

// Delete success path: create a feedback first, then delete it; the handler
// should return 200 with the operation-success message.
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

	// Listing after the delete should show 404/error; nothing should remain
	listCtx, listRec := newFeedbackRequest(http.MethodGet, "/api/v1/feedback?page=1&pageSize=10", "")
	handler.List(listCtx)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listResp FeedbackListResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	assert.Empty(t, listResp.Items)
}
