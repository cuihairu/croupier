package faq

import (
	"context"
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

var faqDBSeq uint64

func newFAQTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("faq_%d", atomic.AddUint64(&faqDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newFAQHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{FAQModel: model.NewFAQModel(db)}
	return NewHandler(NewService(svcCtx))
}

func newFAQRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertFAQErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "message")
}

func TestHandler_List_Empty_Success(t *testing.T) {
	handler := newFAQHandler(newFAQTestDB(t))

	ctx, rec := newFAQRequest(http.MethodGet, "/api/v1/faqs?page=1&pageSize=10", "")
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp FAQListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
	assert.Equal(t, int64(0), resp.Total)
}

func TestHandler_CreateAndGet_RoundTrip(t *testing.T) {
	db := newFAQTestDB(t)
	handler := newFAQHandler(db)

	createCtx, createRec := newFAQRequest(http.MethodPost, "/api/v1/faqs",
		`{"question":"What is Croupier?","answer":"A GM backend","category":"general","tags":["intro"],"visible":true}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code, createRec.Body.String())

	var created FAQCreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	assert.Equal(t, "What is Croupier?", created.Question)
	assert.NotZero(t, created.Id)

	// List reflects the new FAQ.
	listCtx, listRec := newFAQRequest(http.MethodGet, "/api/v1/faqs?page=1&pageSize=10", "")
	handler.List(listCtx)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listResp FAQListResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, created.Id, listResp.Items[0].Id)

	// Delete the FAQ.
	delCtx, delRec := newFAQRequest(http.MethodDelete, fmt.Sprintf("/api/v1/faqs/%d", created.Id), "")
	delCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(created.Id)}}
	handler.Delete(delCtx)
	assert.Equal(t, http.StatusOK, delRec.Code, delRec.Body.String())
}

func TestHandler_Create_MissingFields_BadRequest(t *testing.T) {
	handler := newFAQHandler(newFAQTestDB(t))

	tests := []struct {
		name string
		body string
	}{
		{"missing question", `{"answer":"a","category":"c"}`},
		{"missing answer", `{"question":"q","category":"c"}`},
		{"missing category", `{"question":"q","answer":"a"}`},
		{"empty object", `{}`},
		{"invalid json", `not-json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, rec := newFAQRequest(http.MethodPost, "/api/v1/faqs", tt.body)
			handler.Create(ctx)
			assert.NotEqual(t, http.StatusOK, rec.Code, "expected rejection, got 200 body=%s", rec.Body.String())
			assertFAQErrorShape(t, rec)
		})
	}
}

func TestHandler_Delete_InvalidID_BadRequest(t *testing.T) {
	handler := newFAQHandler(newFAQTestDB(t))

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
			ctx, rec := newFAQRequest(http.MethodDelete, "/api/v1/faqs/"+tt.idVal, "")
			ctx.Params = gin.Params{{Key: "id", Value: tt.idVal}}
			handler.Delete(ctx)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assertFAQErrorShape(t, rec)
		})
	}
}

func TestHandler_Categories_Success(t *testing.T) {
	db := newFAQTestDB(t)
	handler := newFAQHandler(db)

	// Seed a FAQ whose category will be counted.
	_, err := NewService(&svc.ServiceContext{FAQModel: model.NewFAQModel(db)}).
		Create(context.Background(), &FAQCreateRequest{
			Question: "q", Answer: "a", Category: "billing", Visible: true,
		})
	require.NoError(t, err)

	ctx, rec := newFAQRequest(http.MethodGet, "/api/v1/faqs/categories", "")
	handler.Categories(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp FAQCategoriesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Items)
}

func TestHandler_Update_NoFields_BadRequest(t *testing.T) {
	db := newFAQTestDB(t)
	handler := newFAQHandler(db)

	// Seed a FAQ to obtain a valid id.
	created, err := NewService(&svc.ServiceContext{FAQModel: model.NewFAQModel(db)}).
		Create(context.Background(), &FAQCreateRequest{Question: "q", Answer: "a", Category: "c", Visible: true})
	require.NoError(t, err)

	ctx, rec := newFAQRequest(http.MethodPut, fmt.Sprintf("/api/v1/faqs/%d", created.Id), `{}`)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(created.Id)}}
	handler.Update(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertFAQErrorShape(t, rec)
}
