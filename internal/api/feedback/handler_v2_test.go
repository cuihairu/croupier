package feedback

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- helpers ----

func newFeedbackTestDBV2(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("fbv2_%d", atomic.AddUint64(&feedbackDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newFeedbackHandlerV2(t *testing.T) *Handler {
	t.Helper()
	db := newFeedbackTestDBV2(t)
	svcCtx := &svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)}
	return NewHandler(NewService(svcCtx))
}

// ---- Handler Tests ----

func TestHandlerV2_Update_Success(t *testing.T) {
	h := newFeedbackHandlerV2(t)
	ctx, rec := newFeedbackRequest(http.MethodPost, "/api/v1/feedback",
		`{"contact":"u@x.com","content":"hi","category":"bug","rating":3}`)
	h.Create(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var created FeedbackCreateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	// Update status + priority + reply via handler
	ctx2, rec2 := newFeedbackRequest(http.MethodPut, "/api/v1/feedback/"+fmt.Sprintf("%d", created.Id), "")
	ctx2.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", created.Id)}}
	ctx2.Request = httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"status":"closed","priority":"high","reply":"thanks"}`))
	ctx2.Request.Header.Set("Content-Type", "application/json")
	h.Update(ctx2)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())

	var updated FeedbackUpdateResponse
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &updated))
	assert.Equal(t, "closed", updated.Status)
	assert.Equal(t, "high", updated.Priority)
	assert.Equal(t, "thanks", updated.Reply)
}

func TestHandlerV2_Update_InvalidURI(t *testing.T) {
	h := newFeedbackHandlerV2(t)
	ctx, rec := newFeedbackRequest(http.MethodPut, "/api/v1/feedback/abc", "")
	ctx.Params = gin.Params{{Key: "id", Value: "abc"}}
	ctx.Request = httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"status":"closed"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	h.Update(ctx)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// ---- Service Tests ----

func TestServiceV2_List_NilModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	_, err := s.List(nil, nil)
	assert.Error(t, err)
}

func TestServiceV2_List_NilRequest(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	resp, err := s.List(nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestServiceV2_List_WithFilters(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	fbModel := model.NewFeedbackModel(db)
	s := NewService(&svc.ServiceContext{FeedbackModel: fbModel})

	// Create entries
	for _, cat := range []string{"bug", "praise", "bug"} {
		err := fbModel.Create(nil, &model.Feedback{
			Contact: "x@x.com", Content: "c", Category: cat, Status: "open", Priority: "normal",
		})
		require.NoError(t, err)
	}

	resp, err := s.List(nil, &FeedbackListRequest{Category: "bug", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(resp.Items), 1)

	resp2, err := s.List(nil, &FeedbackListRequest{Status: "open", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(resp2.Items), 1)

	resp3, err := s.List(nil, &FeedbackListRequest{GameId: "demo", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.NotNil(t, resp3)
}

func TestServiceV2_Create_NilModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	_, err := s.Create(nil, nil)
	assert.Error(t, err)
}

func TestServiceV2_Create_NilRequest(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := s.Create(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "请求体不能为空")
}

func TestServiceV2_Create_EmptyContact(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := s.Create(nil, &FeedbackCreateRequest{Content: "x", Category: "c"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "联系方式不能为空")
}

func TestServiceV2_Create_EmptyContent(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := s.Create(nil, &FeedbackCreateRequest{Contact: "x", Category: "c"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈内容不能为空")
}

func TestServiceV2_Create_EmptyCategory(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := s.Create(nil, &FeedbackCreateRequest{Contact: "x", Content: "c"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈分类不能为空")
}

func TestServiceV2_Create_Success_AllFields(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	resp, err := s.Create(nil, &FeedbackCreateRequest{
		Contact:  "test@test.com",
		Content:  "great",
		Category: "praise",
		Rating:   5,
		GameId:   "demo",
		Env:      "prod",
		PlayerId: "p1",
		Attach:   "http://example.com/img.png",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "test@test.com", resp.Contact)
	assert.Equal(t, "praise", resp.Category)
	assert.Equal(t, 5, resp.Rating)
	assert.Equal(t, "demo", resp.GameId)
	assert.Equal(t, "p1", resp.PlayerId)
	assert.Equal(t, "http://example.com/img.png", resp.Attach)
}

func TestServiceV2_Update_NilModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	_, err := s.Update(nil, nil)
	assert.Error(t, err)
}

func TestServiceV2_Update_NilRequest(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := s.Update(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "请求体不能为空")
}

func TestServiceV2_Update_InvalidID(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := s.Update(nil, &FeedbackUpdateRequest{ID: "abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈ID格式不正确")
}

func TestServiceV2_Update_NoFieldsToUpdate(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := s.Update(nil, &FeedbackUpdateRequest{ID: "1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "没有需要更新的字段")
}

func TestServiceV2_Update_ReplyEmptyString(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})

	createResp, err := s.Create(nil, &FeedbackCreateRequest{
		Contact: "x@x.com", Content: "c", Category: "bug",
	})
	require.NoError(t, err)

	// Update with Reply = " " (trimmed to empty, but original Reply != "" => clear reply path)
	_, err = s.Update(nil, &FeedbackUpdateRequest{
		ID:       fmt.Sprintf("%d", createResp.Id),
		Status:   "closed",
		Reply:    "   ",
		Priority: "high",
	})
	require.NoError(t, err)
}

func TestServiceV2_Update_ReplyClear(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	createResp, err := s.Create(nil, &FeedbackCreateRequest{
		Contact: "x@x.com", Content: "c", Category: "bug",
	})
	require.NoError(t, err)

	// Set reply
	_, err = s.Update(nil, &FeedbackUpdateRequest{
		ID:     fmt.Sprintf("%d", createResp.Id),
		Status: "open",
		Reply:  "got it",
	})
	require.NoError(t, err)

	// Clear reply (explicit empty string — req.Reply != "" but trimmed == "")
	_, err = s.Update(nil, &FeedbackUpdateRequest{
		ID:    fmt.Sprintf("%d", createResp.Id),
		Reply: "",
	})
	// This will fail because there are no fields to update (Status/Priority are empty)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "没有需要更新的字段")
}

func TestServiceV2_Delete_NilModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	err := s.Delete(nil, nil)
	assert.Error(t, err)
}

func TestServiceV2_Delete_NilRequest(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	err := s.Delete(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "请求体不能为空")
}

func TestServiceV2_Delete_InvalidID(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	err := s.Delete(nil, &FeedbackDeleteRequest{ID: "xyz"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈ID格式不正确")
}

func TestServiceV2_Delete_Success(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	createResp, err := s.Create(nil, &FeedbackCreateRequest{
		Contact: "x@x.com", Content: "c", Category: "bug",
	})
	require.NoError(t, err)
	err = s.Delete(nil, &FeedbackDeleteRequest{ID: fmt.Sprintf("%d", createResp.Id)})
	require.NoError(t, err)
}

func TestServiceV2_Stats_NilModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	_, err := s.Stats(nil, nil)
	assert.Error(t, err)
}

func TestServiceV2_Stats_NilRequest_DefaultDays(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	resp, err := s.Stats(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.Total)
}

func TestServiceV2_Stats_WithDays(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	fbModel := model.NewFeedbackModel(db)
	s := NewService(&svc.ServiceContext{FeedbackModel: fbModel})
	for i := 0; i < 3; i++ {
		err := fbModel.Create(nil, &model.Feedback{
			Contact: fmt.Sprintf("u%d@x.com", i), Content: "c", Category: "bug", Status: "open", Priority: "normal",
		})
		require.NoError(t, err)
	}
	resp, err := s.Stats(nil, &FeedbackStatsRequest{Days: 30})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, 3)
}

func TestServiceV2_Stats_ZeroDays(t *testing.T) {
	db := newFeedbackTestDBV2(t)
	s := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	resp, err := s.Stats(nil, &FeedbackStatsRequest{Days: 0})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestBuildFeedback_Nil(t *testing.T) {
	result := buildFeedback(nil)
	assert.Equal(t, Feedback{}, result)
}
