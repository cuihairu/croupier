package terms

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

var termsDBSeq uint64

func newTermsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("terms_%d", atomic.AddUint64(&termsDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newTermsHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{TermDictModel: model.NewTermDictionaryModel(db)}
	return NewHandler(NewService(svcCtx))
}

func newTermsRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertTermsErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "message")
}

func TestHandler_List_Empty_Success(t *testing.T) {
	handler := newTermsHandler(newTermsTestDB(t))

	ctx, rec := newTermsRequest(http.MethodGet, "/api/v1/terms?domain=resource", "")
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp TermsListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
}

func TestHandler_UpsertAndList_RoundTrip(t *testing.T) {
	db := newTermsTestDB(t)
	handler := newTermsHandler(db)

	upsertCtx, upsertRec := newTermsRequest(http.MethodPost, "/api/v1/terms",
		`{"domain":"resource","termKey":"player","alias":"玩家","displayZh":"玩家","displayEn":"Player","order":1}`)
	handler.Upsert(upsertCtx)
	require.Equal(t, http.StatusOK, upsertRec.Code, upsertRec.Body.String())

	var upsertResp TermUpsertResponse
	require.NoError(t, json.Unmarshal(upsertRec.Body.Bytes(), &upsertResp))
	assert.True(t, upsertResp.Ok)

	// List the domain back.
	listCtx, listRec := newTermsRequest(http.MethodGet, "/api/v1/terms?domain=resource", "")
	handler.List(listCtx)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listResp TermsListResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, "player", listResp.Items[0].TermKey)
	assert.Equal(t, "玩家", listResp.Items[0].Alias)
}

func TestHandler_LegacyEntityDomain_BadRequest(t *testing.T) {
	handler := newTermsHandler(newTermsTestDB(t))

	ctx, rec := newTermsRequest(http.MethodGet, "/api/v1/terms?domain=entity", "")
	handler.List(ctx)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertTermsErrorShape(t, rec)
}

func TestHandler_Delete_Success(t *testing.T) {
	db := newTermsTestDB(t)
	handler := newTermsHandler(db)

	// Seed.
	upsertCtx, upsertRec := newTermsRequest(http.MethodPost, "/api/v1/terms",
		`{"domain":"operation","term_key":"create","alias":"新建"}`)
	handler.Upsert(upsertCtx)
	require.Equal(t, http.StatusOK, upsertRec.Code)

	// Delete the seeded term.
	delCtx, delRec := newTermsRequest(http.MethodDelete, "/api/v1/terms",
		`{"domain":"operation","alias":"新建"}`)
	handler.Delete(delCtx)
	require.Equal(t, http.StatusOK, delRec.Code, delRec.Body.String())
	var delResp TermDeleteResponse
	require.NoError(t, json.Unmarshal(delRec.Body.Bytes(), &delResp))
	assert.True(t, delResp.Ok)
}

func TestHandler_Upsert_InvalidJSON_BadRequest(t *testing.T) {
	handler := newTermsHandler(newTermsTestDB(t))

	ctx, rec := newTermsRequest(http.MethodPost, "/api/v1/terms", `not-json`)
	handler.Upsert(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertTermsErrorShape(t, rec)
}

func TestHandler_Delete_InvalidJSON_BadRequest(t *testing.T) {
	handler := newTermsHandler(newTermsTestDB(t))

	ctx, rec := newTermsRequest(http.MethodDelete, "/api/v1/terms", `not-json`)
	handler.Delete(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertTermsErrorShape(t, rec)
}

// DB 故障 + 非法 domain：List/Upsert/Delete 的错误分支（此前 85.7%）。
func TestTerms_ErrorBranches(t *testing.T) {
	db := newTermsTestDB(t)
	handler := newTermsHandler(db)

	// 非法 domain → 400（BadRequest）
	ctx, rec := newTermsRequest(http.MethodPost, "/terms",
		`{"domain":"nope","alias":"a","content":"c"}`)
	handler.Upsert(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	ctx2, rec2 := newTermsRequest(http.MethodDelete, "/terms",
		`{"domain":"nope","alias":"a"}`)
	handler.Delete(ctx2)
	assert.Equal(t, http.StatusBadRequest, rec2.Code)

	// 删表 → service 错误分支
	require.NoError(t, db.Migrator().DropTable("term_dictionary"))

	ctx3, rec3 := newTermsRequest(http.MethodGet, "/terms?domain=champion", "")
	handler.List(ctx3)
	assert.NotEqual(t, http.StatusOK, rec3.Code)

	ctx4, rec4 := newTermsRequest(http.MethodDelete, "/terms",
		`{"domain":"champion","alias":"a"}`)
	handler.Delete(ctx4)
	assert.NotEqual(t, http.StatusOK, rec4.Code)
}
