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

var feedbackDBSeq uint64

func newFeedbackTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("feedback_%d", atomic.AddUint64(&feedbackDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newFeedbackHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{FeedbackModel: model.NewFeedbackModel(db)}
	return NewHandler(NewService(svcCtx))
}

func newFeedbackRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertFeedbackErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "message")
}

func TestHandler_List_Empty_Success(t *testing.T) {
	handler := newFeedbackHandler(newFeedbackTestDB(t))

	ctx, rec := newFeedbackRequest(http.MethodGet, "/api/v1/feedback?page=1&pageSize=10", "")
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp FeedbackListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
	assert.Equal(t, int64(0), resp.Total)
}

func TestHandler_CreateAndList_RoundTrip(t *testing.T) {
	db := newFeedbackTestDB(t)
	handler := newFeedbackHandler(db)

	createCtx, createRec := newFeedbackRequest(http.MethodPost, "/api/v1/feedback",
		`{"contact":"player1@example.com","content":"great service","category":"praise","rating":5,"gameId":"demo","env":"prod"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code, createRec.Body.String())

	var created FeedbackCreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	assert.Equal(t, "player1@example.com", created.Contact)
	assert.Equal(t, "praise", created.Category)

	// List reflects the new feedback.
	listCtx, listRec := newFeedbackRequest(http.MethodGet, "/api/v1/feedback?page=1&pageSize=10", "")
	handler.List(listCtx)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listResp FeedbackListResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, created.Id, listResp.Items[0].Id)
}

func TestHandler_Create_MissingFields_BadRequest(t *testing.T) {
	handler := newFeedbackHandler(newFeedbackTestDB(t))

	tests := []struct {
		name string
		body string
	}{
		{"missing contact", `{"content":"c","category":"cat"}`},
		{"missing content", `{"contact":"c","category":"cat"}`},
		{"missing category", `{"contact":"c","content":"c"}`},
		{"empty object", `{}`},
		{"invalid json", `not-json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, rec := newFeedbackRequest(http.MethodPost, "/api/v1/feedback", tt.body)
			handler.Create(ctx)
			assert.NotEqual(t, http.StatusOK, rec.Code, "expected rejection, got 200 body=%s", rec.Body.String())
			assertFeedbackErrorShape(t, rec)
		})
	}
}

func TestHandler_Delete_InvalidID_Rejected(t *testing.T) {
	handler := newFeedbackHandler(newFeedbackTestDB(t))

	// Note: the feedback service validates the id with strconv.ParseUint and
	// returns a plain error (not an errorx.CodeError), so invalid ids surface
	// as 500/internal_error rather than 400. The contract guarantee we lock
	// down here is that the request is rejected (never 200) and uses the
	// unified error shape.
	tests := []struct {
		name  string
		idVal string
	}{
		{"empty", ""},
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, rec := newFeedbackRequest(http.MethodDelete, "/api/v1/feedback/"+tt.idVal, "")
			ctx.Params = gin.Params{{Key: "id", Value: tt.idVal}}
			handler.Delete(ctx)
			assert.NotEqual(t, http.StatusOK, rec.Code, "expected rejection, got 200 body=%s", rec.Body.String())
			assertFeedbackErrorShape(t, rec)
		})
	}
}

func TestHandler_Delete_NotFound(t *testing.T) {
	handler := newFeedbackHandler(newFeedbackTestDB(t))

	// Delete first checks FindByID, so a missing-but-valid id surfaces an error.
	ctx, rec := newFeedbackRequest(http.MethodDelete, "/api/v1/feedback/99999", "")
	ctx.Params = gin.Params{{Key: "id", Value: "99999"}}
	handler.Delete(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertFeedbackErrorShape(t, rec)
}

func TestHandler_Stats_Success(t *testing.T) {
	db := newFeedbackTestDB(t)
	handler := newFeedbackHandler(db)

	// Seed feedback so stats has data.
	createCtx, createRec := newFeedbackRequest(http.MethodPost, "/api/v1/feedback",
		`{"contact":"p@x.com","content":"hi","category":"praise","rating":4}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code, createRec.Body.String())
	var created FeedbackCreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	assert.NotZero(t, created.Id)

	ctx, rec := newFeedbackRequest(http.MethodGet, "/api/v1/feedback/stats?days=7", "")
	handler.Stats(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp FeedbackStatsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.GreaterOrEqual(t, resp.Total, 1)
}

func TestService_NilModel_Error(t *testing.T) {
	// A ServiceContext without a FeedbackModel must surface a clean error.
	svc := NewService(&svc.ServiceContext{})
	_, err := svc.List(nil, &FeedbackListRequest{})
	assert.Error(t, err)
}
