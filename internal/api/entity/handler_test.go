package entity

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

var entityDBSeq uint64

func newEntityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("entity_%d", atomic.AddUint64(&entityDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newEntityHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{EntityModel: model.NewEntityModel(db)}
	return NewHandler(NewService(svcCtx))
}

func newEntityRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertEntityErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "message")
}

func TestHandler_List_Empty_Success(t *testing.T) {
	handler := newEntityHandler(newEntityTestDB(t))

	ctx, rec := newEntityRequest(http.MethodGet, "/api/v1/entities?page=1&pageSize=10", "")
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp EntitiesListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
	assert.Equal(t, int64(0), resp.Total)
}

func TestHandler_CreateAndGet_RoundTrip(t *testing.T) {
	db := newEntityTestDB(t)
	handler := newEntityHandler(db)

	createCtx, createRec := newEntityRequest(http.MethodPost, "/api/v1/entities",
		`{"type":"player","data":{"name":"alice","level":7}}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code, createRec.Body.String())

	var created EntityCreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	assert.Equal(t, "player", created.Type)
	assert.NotEmpty(t, created.ID)

	// Fetch the created entity back by id.
	getCtx, getRec := newEntityRequest(http.MethodGet, "/api/v1/entities/"+created.ID, "")
	getCtx.Params = gin.Params{{Key: "id", Value: created.ID}}
	handler.Detail(getCtx)

	require.Equal(t, http.StatusOK, getRec.Code, getRec.Body.String())
	var detail EntityDetailResponse
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &detail))
	assert.Equal(t, created.ID, detail.ID)
	assert.Equal(t, "player", detail.Type)

	// List should now contain at least one entity.
	listCtx, listRec := newEntityRequest(http.MethodGet, "/api/v1/entities?page=1&pageSize=10", "")
	handler.List(listCtx)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listResp EntitiesListResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	assert.GreaterOrEqual(t, listResp.Total, int64(1))
	require.NotEmpty(t, listResp.Items)
	assert.Equal(t, "player", listResp.Items[0].Type)
}

func TestHandler_Create_MissingType_BadRequest(t *testing.T) {
	handler := newEntityHandler(newEntityTestDB(t))

	tests := []struct {
		name string
		body string
	}{
		{"missing type", `{"data":{"a":1}}`},
		{"missing data", `{"type":"player"}`},
		{"empty object", `{}`},
		{"invalid json", `not-json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, rec := newEntityRequest(http.MethodPost, "/api/v1/entities", tt.body)
			handler.Create(ctx)
			assert.NotEqual(t, http.StatusOK, rec.Code, "expected rejection, got 200 body=%s", rec.Body.String())
			assertEntityErrorShape(t, rec)
		})
	}
}

func TestHandler_Detail_InvalidID_BadRequest(t *testing.T) {
	handler := newEntityHandler(newEntityTestDB(t))

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
			ctx, rec := newEntityRequest(http.MethodGet, "/api/v1/entities/"+tt.idVal, "")
			ctx.Params = gin.Params{{Key: "id", Value: tt.idVal}}
			handler.Detail(ctx)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assertEntityErrorShape(t, rec)
		})
	}
}

func TestHandler_Detail_NotFound(t *testing.T) {
	handler := newEntityHandler(newEntityTestDB(t))

	ctx, rec := newEntityRequest(http.MethodGet, "/api/v1/entities/99999", "")
	ctx.Params = gin.Params{{Key: "id", Value: "99999"}}
	handler.Detail(ctx)

	// Non-existent valid id surfaces a model error (not a 200).
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertEntityErrorShape(t, rec)
}

func TestHandler_Delete_Success(t *testing.T) {
	db := newEntityTestDB(t)
	handler := newEntityHandler(db)

	// Seed an entity through the service.
	created, err := NewService(&svc.ServiceContext{EntityModel: model.NewEntityModel(db)}).
		Create(context.Background(), &EntityCreateRequest{Type: "player", Data: map[string]interface{}{"k": "v"}})
	require.NoError(t, err)

	ctx, rec := newEntityRequest(http.MethodDelete, "/api/v1/entities/"+created.ID, "")
	ctx.Params = gin.Params{{Key: "id", Value: created.ID}}
	handler.Delete(ctx)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_Validate_Success(t *testing.T) {
	handler := newEntityHandler(newEntityTestDB(t))

	ctx, rec := newEntityRequest(http.MethodPost, "/api/v1/entities/validate",
		`{"type":"player","data":{"x":1}}`)
	handler.Validate(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp EntityValidateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Valid)
}

func TestHandler_Preview_Success(t *testing.T) {
	db := newEntityTestDB(t)
	handler := newEntityHandler(db)

	created, err := NewService(&svc.ServiceContext{EntityModel: model.NewEntityModel(db)}).
		Create(context.Background(), &EntityCreateRequest{Type: "player", Data: map[string]interface{}{"name": "bob"}})
	require.NoError(t, err)

	ctx, rec := newEntityRequest(http.MethodGet, "/api/v1/entities/"+created.ID+"/preview", "")
	ctx.Params = gin.Params{{Key: "id", Value: created.ID}}
	handler.Preview(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp EntityPreviewResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Data)
}
