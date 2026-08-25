package ticket

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

var ticketDBSeq uint64

func newTicketTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("ticket_%d", atomic.AddUint64(&ticketDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newTicketHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{TicketModel: model.NewTicketModel(db)}
	return NewHandler(NewService(svcCtx))
}

func newTicketRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertTicketErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "message")
}

func TestHandler_List_Empty_Success(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets?page=1&pageSize=10", "")
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
	assert.Equal(t, int64(0), resp.Total)
}

func TestHandler_CreateAndGet_RoundTrip(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"Login broken","content":"Cannot login","category":"bug","priority":"high","tags":["auth"],"playerId":"p1","contact":"p1@x.com","gameId":"demo","env":"prod"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code, createRec.Body.String())

	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	assert.Equal(t, "Login broken", created.Ticket.Title)
	assert.NotZero(t, created.Ticket.Id)

	idStr := fmt.Sprint(created.Ticket.Id)

	// Fetch the ticket back by id.
	getCtx, getRec := newTicketRequest(http.MethodGet, "/api/v1/tickets/"+idStr, "")
	getCtx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.Get(getCtx)
	require.Equal(t, http.StatusOK, getRec.Code, getRec.Body.String())
	var detail GetResponse
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &detail))
	assert.Equal(t, created.Ticket.Id, detail.Ticket.Id)

	// List reflects the new ticket.
	listCtx, listRec := newTicketRequest(http.MethodGet, "/api/v1/tickets?page=1&pageSize=10", "")
	handler.List(listCtx)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listResp ListResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, created.Ticket.Id, listResp.Items[0].Id)
}

func TestHandler_Create_MissingFields_BadRequest(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	tests := []struct {
		name string
		body string
	}{
		{"missing title", `{"content":"c","category":"bug"}`},
		{"missing content", `{"title":"t","category":"bug"}`},
		{"missing category", `{"title":"t","content":"c"}`},
		{"empty object", `{}`},
		{"invalid json", `not-json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, rec := newTicketRequest(http.MethodPost, "/api/v1/tickets", tt.body)
			handler.Create(ctx)
			assert.NotEqual(t, http.StatusOK, rec.Code, "expected rejection, got 200 body=%s", rec.Body.String())
			assertTicketErrorShape(t, rec)
		})
	}
}

func TestHandler_Get_InvalidID_BadRequest(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

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
			ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets/"+tt.idVal, "")
			ctx.Params = gin.Params{{Key: "id", Value: tt.idVal}}
			handler.Get(ctx)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assertTicketErrorShape(t, rec)
		})
	}
}

func TestHandler_Get_NotFound(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets/99999", "")
	ctx.Params = gin.Params{{Key: "id", Value: "99999"}}
	handler.Get(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertTicketErrorShape(t, rec)
}

func TestHandler_Transition_Success(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	// Seed a ticket.
	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)
	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	idStr := fmt.Sprint(created.Ticket.Id)

	// Transition uses ShouldBindJSON; id is supplied in the body.
	transitionCtx, transitionRec := newTicketRequest(http.MethodPost, "/api/v1/tickets/"+idStr+"/transition",
		fmt.Sprintf(`{"id":"%s","status":"in_progress","note":"picking up"}`, idStr))
	transitionCtx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.Transition(transitionCtx)

	require.Equal(t, http.StatusOK, transitionRec.Code, transitionRec.Body.String())
	var resp TransitionResponse
	require.NoError(t, json.Unmarshal(transitionRec.Body.Bytes(), &resp))
	assert.Equal(t, "in_progress", resp.Ticket.Status)
}

func TestHandler_Transition_InvalidStatus_BadRequest(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)
	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	idStr := fmt.Sprint(created.Ticket.Id)

	ctx, rec := newTicketRequest(http.MethodPost, "/api/v1/tickets/"+idStr+"/transition",
		fmt.Sprintf(`{"id":"%s","status":"bogus"}`, idStr))
	ctx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.Transition(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertTicketErrorShape(t, rec)
}

func TestHandler_CreateComment_AndList(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)
	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	idStr := fmt.Sprint(created.Ticket.Id)

	// CreateComment binds JSON; ticketId is supplied in the body.
	commentCtx, commentRec := newTicketRequest(http.MethodPost, "/api/v1/tickets/"+idStr+"/comments",
		fmt.Sprintf(`{"ticketId":"%s","content":"looks related to auth"}`, idStr))
	commentCtx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.CreateComment(commentCtx)
	require.Equal(t, http.StatusOK, commentRec.Code, commentRec.Body.String())
	var commentResp CreateCommentResponse
	require.NoError(t, json.Unmarshal(commentRec.Body.Bytes(), &commentResp))
	require.Len(t, commentResp.Items, 1)

	// Fetch comments via GetComments (uri: ticketId).
	getCommentsCtx, getCommentsRec := newTicketRequest(http.MethodGet, "/api/v1/tickets/"+idStr+"/comments", "")
	getCommentsCtx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.GetComments(getCommentsCtx)
	require.Equal(t, http.StatusOK, getCommentsRec.Code, getCommentsRec.Body.String())
	var gcResp GetCommentsResponse
	require.NoError(t, json.Unmarshal(getCommentsRec.Body.Bytes(), &gcResp))
	require.Len(t, gcResp.Items, 1)
	assert.Equal(t, "looks related to auth", gcResp.Items[0].Content)
}

func TestHandler_Delete_Success(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)
	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	idStr := fmt.Sprint(created.Ticket.Id)

	ctx, rec := newTicketRequest(http.MethodDelete, "/api/v1/tickets/"+idStr, "")
	ctx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.Delete(ctx)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_Delete_InvalidID_BadRequest(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodDelete, "/api/v1/tickets/abc", "")
	ctx.Params = gin.Params{{Key: "id", Value: "abc"}}
	handler.Delete(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertTicketErrorShape(t, rec)
}
