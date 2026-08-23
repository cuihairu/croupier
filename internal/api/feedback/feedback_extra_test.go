package feedback

import (
	"context"
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

var feedbackExtraDBSeq uint64

func newFeedbackExtraTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("feedback_extra_%d", atomic.AddUint64(&feedbackExtraDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newFeedbackExtraHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)}
	return NewHandler(NewService(svcCtx))
}

func newFeedbackExtraRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

// Test currentFeedbackScope function
func TestCurrentFeedbackScope_Success(t *testing.T) {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game1", Env: "prod"})
	scope, err := currentFeedbackScope(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "game1", scope.GameID)
	assert.Equal(t, "prod", scope.Env)
}

func TestCurrentFeedbackScope_EmptyContext(t *testing.T) {
	// Empty context should return an error since scope is missing
	_, err := currentFeedbackScope(context.Background())
	assert.Error(t, err)
}

// Test requireFeedbackScope function
func TestRequireFeedbackScope_NilFeedback(t *testing.T) {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game1", Env: "prod"})
	err := requireFeedbackScope(ctx, nil)
	assert.Error(t, err)
}

func TestRequireFeedbackScope_MatchingScope(t *testing.T) {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game1", Env: "prod"})
	feedback := &model.Feedback{GameID: "game1", Env: "prod"}
	err := requireFeedbackScope(ctx, feedback)
	assert.NoError(t, err)
}

func TestRequireFeedbackScope_NonMatchingScope(t *testing.T) {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game1", Env: "prod"})
	feedback := &model.Feedback{GameID: "game2", Env: "dev"}
	err := requireFeedbackScope(ctx, feedback)
	assert.Error(t, err)
}

func TestRequireFeedbackScope_EmptyScope(t *testing.T) {
	ctx := context.Background()
	feedback := &model.Feedback{GameID: "game1", Env: "prod"}
	err := requireFeedbackScope(ctx, feedback)
	assert.NoError(t, err)
}

// Test buildFeedback function
func TestBuildFeedback_NilRecord(t *testing.T) {
	result := buildFeedback(nil)
	assert.Equal(t, Feedback{}, result)
}

func TestBuildFeedback_WithRecord(t *testing.T) {
	record := &model.Feedback{
		PlayerID: "player1",
		Contact:  "test@example.com",
		Content:  "test content",
		Category: "bug",
		Priority: "high",
		Status:   "open",
		Rating:   5,
		Reply:    "thanks",
		GameID:   "game1",
		Env:      "prod",
	}
	result := buildFeedback(record)
	assert.Equal(t, "player1", result.PlayerId)
	assert.Equal(t, "test@example.com", result.Contact)
	assert.Equal(t, "test content", result.Content)
	assert.Equal(t, "bug", result.Category)
	assert.Equal(t, "high", result.Priority)
	assert.Equal(t, "open", result.Status)
	assert.Equal(t, 5, result.Rating)
	assert.Equal(t, "thanks", result.Reply)
	assert.Equal(t, "game1", result.GameId)
	assert.Equal(t, "prod", result.Env)
}

// Test service methods with nil model
func TestService_List_NilModel(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	_, err := service.List(context.Background(), &FeedbackListRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈模型未初始化")
}

func TestService_Create_NilModel(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	_, err := service.Create(context.Background(), &FeedbackCreateRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈模型未初始化")
}

func TestService_Update_NilModel(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	_, err := service.Update(context.Background(), &FeedbackUpdateRequest{ID: "1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈模型未初始化")
}

func TestService_Delete_NilModel(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	err := service.Delete(context.Background(), &FeedbackDeleteRequest{ID: "1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈模型未初始化")
}

func TestService_Stats_NilModel(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	_, err := service.Stats(context.Background(), &FeedbackStatsRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈模型未初始化")
}

// Test validation errors
func TestService_Create_NilRequest(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := service.Create(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "请求体不能为空")
}

func TestService_Create_EmptyContact(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := service.Create(context.Background(), &FeedbackCreateRequest{Content: "test", Category: "bug"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "联系方式不能为空")
}

func TestService_Create_EmptyContent(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := service.Create(context.Background(), &FeedbackCreateRequest{Contact: "test@example.com", Category: "bug"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈内容不能为空")
}

func TestService_Create_EmptyCategory(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := service.Create(context.Background(), &FeedbackCreateRequest{Contact: "test@example.com", Content: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈分类不能为空")
}

// Test update validation
func TestService_Update_NilRequest(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := service.Update(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "请求体不能为空")
}

func TestService_Update_InvalidID(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	_, err := service.Update(context.Background(), &FeedbackUpdateRequest{ID: "invalid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈ID格式不正确")
}

// Test delete validation
func TestService_Delete_NilRequest(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	err := service.Delete(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "请求体不能为空")
}

func TestService_Delete_InvalidID(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	err := service.Delete(context.Background(), &FeedbackDeleteRequest{ID: "invalid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "反馈ID格式不正确")
}

// Test stats defaults
func TestService_Stats_NilRequest(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	resp, err := service.Stats(context.Background(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Stats returns total count of feedbacks, not days
	assert.Equal(t, 0, resp.Total)
}

func TestService_Stats_CustomDays(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})
	resp, err := service.Stats(context.Background(), &FeedbackStatsRequest{Days: 30})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// Test list with filters
func TestService_List_WithFilters(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	service := NewService(&svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)})

	// Create some feedback first
	_, _ = service.Create(context.Background(), &FeedbackCreateRequest{
		Contact:  "test@example.com",
		Content:  "test content",
		Category: "bug",
		Rating:   5,
		GameId:   "game1",
		Env:      "prod",
	})

	resp, err := service.List(context.Background(), &FeedbackListRequest{
		Page:     1,
		PageSize: 10,
		Status:   "open",
		Category: "bug",
		GameId:   "game1",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 10, resp.Size)
}

// Test handler with invalid query params
func TestHandler_List_InvalidQuery(t *testing.T) {
	db := newFeedbackExtraTestDB(t)
	handler := newFeedbackExtraHandler(db)

	ctx, rec := newFeedbackExtraRequest(http.MethodGet, "/api/v1/feedback?page=invalid", "")
	handler.List(ctx)

	// Should handle invalid query params gracefully
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}
