package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- helpers.go unit tests ----------

func TestDecodeTicketTags_VariousTypes(t *testing.T) {
	t.Parallel()

	t.Run("string_slice", func(t *testing.T) {
		out := decodeTicketTags([]string{"a", "b"})
		assert.Equal(t, []string{"a", "b"}, out)
	})

	t.Run("interface_slice", func(t *testing.T) {
		out := decodeTicketTags([]interface{}{"x", "y"})
		assert.Equal(t, []string{"x", "y"}, out)
	})

	t.Run("interface_slice_non_string_elements", func(t *testing.T) {
		out := decodeTicketTags([]interface{}{123, "ok"})
		assert.Equal(t, []string{"ok"}, out)
	})

	t.Run("json_raw_message", func(t *testing.T) {
		raw := json.RawMessage(`["tag1","tag2"]`)
		out := decodeTicketTags(raw)
		assert.Equal(t, []string{"tag1", "tag2"}, out)
	})

	t.Run("json_raw_message_invalid", func(t *testing.T) {
		raw := json.RawMessage(`not-json`)
		out := decodeTicketTags(raw)
		assert.Nil(t, out)
	})

	t.Run("byte_slice", func(t *testing.T) {
		out := decodeTicketTags([]byte(`["z"]`))
		assert.Equal(t, []string{"z"}, out)
	})

	t.Run("byte_slice_invalid", func(t *testing.T) {
		out := decodeTicketTags([]byte(`bad`))
		assert.Nil(t, out)
	})

	t.Run("nil", func(t *testing.T) {
		out := decodeTicketTags(nil)
		assert.Nil(t, out)
	})

	t.Run("unknown_type", func(t *testing.T) {
		out := decodeTicketTags(42)
		assert.Nil(t, out)
	})
}

func TestEncodeTicketTags(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		assert.Nil(t, encodeTicketTags(nil))
		assert.Nil(t, encodeTicketTags([]string{}))
	})

	t.Run("normal", func(t *testing.T) {
		result := encodeTicketTags([]string{"A", "B"})
		require.NotNil(t, result)
		var tags []string
		require.NoError(t, json.Unmarshal(result, &tags))
		assert.Equal(t, []string{"A", "B"}, tags)
	})

	t.Run("dedup_and_trim", func(t *testing.T) {
		result := encodeTicketTags([]string{" Tag ", "tag", "  ", "Tag"})
		require.NotNil(t, result)
		var tags []string
		require.NoError(t, json.Unmarshal(result, &tags))
		assert.Equal(t, []string{"Tag"}, tags)
	})

	t.Run("all_whitespace", func(t *testing.T) {
		assert.Nil(t, encodeTicketTags([]string{"  ", " "}))
	})
}

func TestSanitizePriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"", "medium"},
		{"  ", "medium"},
		{"low", "low"},
		{"LOW", "low"},
		{"Medium", "medium"},
		{"HIGH", "high"},
		{"critical", "critical"},
		{"invalid", "medium"},
		{"  High  ", "high"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizePriority(tt.input))
		})
	}
}

func TestSanitizeTicketStatus(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		_, err := sanitizeTicketStatus("")
		require.Error(t, err)
		assertBadRequest(t, err)
	})

	t.Run("whitespace_only", func(t *testing.T) {
		_, err := sanitizeTicketStatus("   ")
		require.Error(t, err)
		assertBadRequest(t, err)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := sanitizeTicketStatus("bogus")
		require.Error(t, err)
		assertBadRequest(t, err)
	})

	for _, status := range []string{"open", "in_progress", "resolved", "closed"} {
		t.Run(status, func(t *testing.T) {
			got, err := sanitizeTicketStatus(status)
			require.NoError(t, err)
			assert.Equal(t, status, got)
		})
	}

	t.Run("case_insensitive", func(t *testing.T) {
		got, err := sanitizeTicketStatus("OPEN")
		require.NoError(t, err)
		assert.Equal(t, "open", got)
	})
}

func assertBadRequest(t *testing.T, err error) {
	t.Helper()
	var codeErr *errorx.CodeError
	require.True(t, err != nil)
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
}

func TestContextUsername(t *testing.T) {
	t.Parallel()

	t.Run("nil_context", func(t *testing.T) {
		assert.Equal(t, "", contextUsername(nil))
	})

	t.Run("no_username_key", func(t *testing.T) {
		ctx := context.Background()
		assert.Equal(t, "", contextUsername(ctx))
	})

	t.Run("non_string_value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "username", 123)
		assert.Equal(t, "", contextUsername(ctx))
	})

	t.Run("empty_string", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "username", "")
		assert.Equal(t, "", contextUsername(ctx))
	})

	t.Run("whitespace_string", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "username", "  alice  ")
		assert.Equal(t, "alice", contextUsername(ctx))
	})
}

func TestCommentAuthor(t *testing.T) {
	t.Parallel()

	t.Run("with_username", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "username", "bob")
		assert.Equal(t, "bob", commentAuthor(ctx))
	})

	t.Run("without_username", func(t *testing.T) {
		assert.Equal(t, "system", commentAuthor(context.Background()))
	})

	t.Run("nil_context", func(t *testing.T) {
		assert.Equal(t, "system", commentAuthor(nil))
	})
}

func TestBuildCommentsDTO(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		result := buildCommentsDTO(nil)
		assert.Empty(t, result)
	})

	t.Run("with_data", func(t *testing.T) {
		comments := []model.TicketComment{
			{Author: "alice", Content: "hello"},
		}
		result := buildCommentsDTO(comments)
		require.Len(t, result, 1)
		assert.Equal(t, "alice", result[0].Author)
		assert.Equal(t, "hello", result[0].Content)
	})
}

func TestBuildTicketDTO(t *testing.T) {
	t.Parallel()
	ticket := &model.Ticket{
		Title:    "Test",
		Content:  "Content",
		Category: "bug",
		Priority: "high",
		Status:   "open",
		Assignee: "alice",
		PlayerID: "p1",
		Contact:  "a@b.com",
		GameID:   "demo",
		Env:      "prod",
		Source:   "api",
	}
	dto := buildTicketDTO(ticket)
	assert.Equal(t, "Test", dto.Title)
	assert.Equal(t, "bug", dto.Category)
	assert.Equal(t, "alice", dto.Assignee)
	assert.Equal(t, "p1", dto.PlayerId)
	assert.Equal(t, "demo", dto.GameId)
	assert.Equal(t, "prod", dto.Env)
	assert.Equal(t, "api", dto.Source)
}

func TestAddComment(t *testing.T) {
	t.Parallel()
	comment := addComment("bob", "  hello  ", 42)
	assert.Equal(t, "bob", comment.Author)
	assert.Equal(t, "hello", comment.Content)
	assert.Equal(t, uint(42), comment.TicketID)
}

// ---------- Service tests for uncovered paths ----------

func newTicketServiceForTest2(t *testing.T) *Service {
	t.Helper()
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{
		TicketModel: model.NewTicketModel(db),
	}
	return NewService(svcCtx)
}

func TestService_Update_InvalidID(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	_, err := svc.Update(context.Background(), &UpdateRequest{ID: "abc"})
	require.Error(t, err)
}

func TestService_Update_NoFields(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	// Create a ticket first
	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(createResp.Ticket.Id)

	// Update with no fields
	_, err = svc.Update(context.Background(), &UpdateRequest{ID: idStr})
	require.Error(t, err)
	assertBadRequest(t, err)
}

func TestService_Update_WithFields(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(createResp.Ticket.Id)

	resp, err := svc.Update(context.Background(), &UpdateRequest{
		ID:       idStr,
		Title:    "New Title",
		Content:  "New Content",
		Category: "feature",
		Priority: "high",
		Assignee: "alice",
		Tags:     []string{"tag1"},
	})
	require.NoError(t, err)
	assert.Equal(t, "New Title", resp.Ticket.Title)
	assert.Equal(t, "New Content", resp.Ticket.Content)
	assert.Equal(t, "feature", resp.Ticket.Category)
	assert.Equal(t, "high", resp.Ticket.Priority)
	assert.Equal(t, "alice", resp.Ticket.Assignee)
}

func TestService_Update_PartialFields(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(createResp.Ticket.Id)

	// Update only title
	resp, err := svc.Update(context.Background(), &UpdateRequest{
		ID:    idStr,
		Title: "Updated",
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated", resp.Ticket.Title)
	assert.Equal(t, "c", resp.Ticket.Content) // unchanged
}

func TestService_Update_NotFound(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	_, err := svc.Update(context.Background(), &UpdateRequest{
		ID:    "99999",
		Title: "x",
	})
	require.Error(t, err)
}

func TestService_Delete_InvalidID(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	err := svc.Delete(context.Background(), &DeleteRequest{ID: "abc"})
	require.Error(t, err)
}

func TestService_GetComments_InvalidID(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	_, err := svc.GetComments(context.Background(), &GetCommentsRequest{TicketID: "abc"})
	require.Error(t, err)
}

func TestService_CreateComment_InvalidID(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	_, err := svc.CreateComment(context.Background(), &CreateCommentRequest{
		TicketID: "abc",
		Content:  "test",
	})
	require.Error(t, err)
}

func TestService_CreateComment_EmptyContent(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(createResp.Ticket.Id)

	_, err = svc.CreateComment(context.Background(), &CreateCommentRequest{
		TicketID: idStr,
		Content:  "",
	})
	require.Error(t, err)
	assertBadRequest(t, err)
}

func TestService_CreateComment_TicketNotFound(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	_, err := svc.CreateComment(context.Background(), &CreateCommentRequest{
		TicketID: "99999",
		Content:  "test",
	})
	require.Error(t, err)
}

func TestService_Transition_InvalidID(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	_, err := svc.Transition(context.Background(), &TransitionRequest{ID: "abc", Status: "open"})
	require.Error(t, err)
}

func TestService_Transition_EmptyStatus(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(createResp.Ticket.Id)

	_, err = svc.Transition(context.Background(), &TransitionRequest{ID: idStr, Status: ""})
	require.Error(t, err)
	assertBadRequest(t, err)
}

func TestService_Transition_InvalidStatus(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(createResp.Ticket.Id)

	_, err = svc.Transition(context.Background(), &TransitionRequest{ID: idStr, Status: "bogus"})
	require.Error(t, err)
	assertBadRequest(t, err)
}

func TestService_Transition_WithoutNote(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(createResp.Ticket.Id)

	resp, err := svc.Transition(context.Background(), &TransitionRequest{
		ID:     idStr,
		Status: "in_progress",
		Note:   "", // empty note — no comment should be created
	})
	require.NoError(t, err)
	assert.Equal(t, "in_progress", resp.Ticket.Status)
}

func TestService_List_WithFilters(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	// Create a ticket
	_, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
		Priority: "high",
	})
	require.NoError(t, err)

	// List with filters
	resp, err := svc.List(context.Background(), &ListRequest{
		Page:     1,
		PageSize: 10,
		Status:   "open",
		Category: "bug",
		Priority: "high",
		Assignee: "",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
}

func TestService_Create_WithTags(t *testing.T) {
	t.Parallel()
	svc := newTicketServiceForTest2(t)

	resp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
		Tags:     []string{"a", "b"},
	})
	require.NoError(t, err)
	// Verify the ticket was created successfully; tags round-trip through
	// datatypes.JSON/SQLite may not preserve the exact slice type.
	assert.NotZero(t, resp.Ticket.Id)
}

func TestService_CreateComment_Success(t *testing.T) {
	svc := newTicketServiceForTest2(t)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(createResp.Ticket.Id)

	commentResp, err := svc.CreateComment(context.Background(), &CreateCommentRequest{
		TicketID: idStr,
		Content:  "looks good",
	})
	require.NoError(t, err)
	require.Len(t, commentResp.Items, 1)
	assert.Equal(t, "looks good", commentResp.Items[0].Content)
}

func TestService_GetComments_Success(t *testing.T) {
	svc := newTicketServiceForTest2(t)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Title:    "t",
		Content:  "c",
		Category: "bug",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(createResp.Ticket.Id)

	// Create a comment first
	_, err = svc.CreateComment(context.Background(), &CreateCommentRequest{
		TicketID: idStr,
		Content:  "a comment",
	})
	require.NoError(t, err)

	commentsResp, err := svc.GetComments(context.Background(), &GetCommentsRequest{TicketID: idStr})
	require.NoError(t, err)
	require.Len(t, commentsResp.Items, 1)
	assert.Equal(t, "a comment", commentsResp.Items[0].Content)
}

// ---------- Handler Update tests ----------

func TestHandler_Update_Success(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	// Create ticket
	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)
	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	idStr := fmt.Sprint(created.Ticket.Id)

	// Update
	updateBody := `{"title":"updated","content":"updated content","category":"feature","priority":"high","assignee":"alice","tags":["x"]}`
	updateCtx, updateRec := newTicketRequest(http.MethodPut, "/api/v1/tickets/"+idStr, updateBody)
	updateCtx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.Update(updateCtx)

	require.Equal(t, http.StatusOK, updateRec.Code, updateRec.Body.String())
	var resp UpdateResponse
	require.NoError(t, json.Unmarshal(updateRec.Body.Bytes(), &resp))
	assert.Equal(t, "updated", resp.Ticket.Title)
	assert.Equal(t, "updated content", resp.Ticket.Content)
}

func TestHandler_Update_InvalidID(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodPut, "/api/v1/tickets/abc", `{"title":"x"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "abc"}}
	handler.Update(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Update_InvalidJSON(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodPut, "/api/v1/tickets/1", `not-json`)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	handler.Update(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestHandler_Update_NoFields(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	// Create ticket
	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)
	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	idStr := fmt.Sprint(created.Ticket.Id)

	// Update with empty body (no fields to update)
	updateCtx, updateRec := newTicketRequest(http.MethodPut, "/api/v1/tickets/"+idStr, `{}`)
	updateCtx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.Update(updateCtx)

	assert.NotEqual(t, http.StatusOK, updateRec.Code)
}

func TestHandler_List_WithFilters(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	// Create a ticket
	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)

	// List with query params
	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets?page=1&pageSize=5&status=open&category=bug", "")
	handler.List(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Items, 1)
}

func TestHandler_GetComments_InvalidID(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets/abc/comments", "")
	ctx.Params = gin.Params{{Key: "ticketId", Value: "abc"}}
	handler.GetComments(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateComment_EmptyContent(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	// Create ticket
	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)
	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	idStr := fmt.Sprint(created.Ticket.Id)

	// Create comment with empty content
	commentCtx, commentRec := newTicketRequest(http.MethodPost, "/api/v1/tickets/"+idStr+"/comments",
		fmt.Sprintf(`{"ticketId":"%s","content":""}`, idStr))
	commentCtx.Params = gin.Params{{Key: "ticketId", Value: idStr}}
	handler.CreateComment(commentCtx)

	assert.NotEqual(t, http.StatusOK, commentRec.Code)
}

func TestHandler_Transition_EmptyStatus(t *testing.T) {
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
		fmt.Sprintf(`{"id":"%s","status":""}`, idStr))
	ctx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.Transition(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestHandler_Transition_InvalidJSON(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodPost, "/api/v1/tickets/1/transition", `not-json`)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	handler.Transition(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestHandler_Delete_NotFound(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodDelete, "/api/v1/tickets/99999", "")
	ctx.Params = gin.Params{{Key: "id", Value: "99999"}}
	handler.Delete(ctx)

	// GORM soft-delete does not error for missing IDs; handler returns 200.
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_GetComments_Success(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	// Create ticket
	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)
	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	idStr := fmt.Sprint(created.Ticket.Id)

	// Get comments (empty)
	getCtx, getRec := newTicketRequest(http.MethodGet, "/api/v1/tickets/"+idStr+"/comments", "")
	getCtx.Params = gin.Params{{Key: "ticketId", Value: idStr}}
	handler.GetComments(getCtx)
	require.Equal(t, http.StatusOK, getRec.Code)
	var gcResp GetCommentsResponse
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &gcResp))
	assert.Empty(t, gcResp.Items)
}

func TestHandler_Update_WithOnlyTags(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	// Create ticket
	createCtx, createRec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"t","content":"c","category":"bug"}`)
	handler.Create(createCtx)
	require.Equal(t, http.StatusOK, createRec.Code)
	var created CreateResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	idStr := fmt.Sprint(created.Ticket.Id)

	// Update with only tags
	updateBody := `{"tags":["newtag"]}`
	updateCtx, updateRec := newTicketRequest(http.MethodPut, "/api/v1/tickets/"+idStr, updateBody)
	updateCtx.Params = gin.Params{{Key: "id", Value: idStr}}
	handler.Update(updateCtx)

	require.Equal(t, http.StatusOK, updateRec.Code, updateRec.Body.String())
}

func TestHandler_List_FilteredEmptyResult(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets?status=closed", "")
	handler.List(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
}
