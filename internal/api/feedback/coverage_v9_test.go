package feedback

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- v9 helpers ----

var v9FbDBSeq uint64

func newFeedbackV9DB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("fbv9_%d", atomic.AddUint64(&v9FbDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

type v9FbFixture struct {
	db  *gorm.DB
	svc *Service
}

func newFeedbackV9Fixture(t *testing.T) *v9FbFixture {
	t.Helper()
	db := newFeedbackV9DB(t)
	svcCtx := &svc.ServiceContext{
		FeedbackModel: model.NewFeedbackModel(db),
		TicketModel:   model.NewTicketModel(db),
	}
	return &v9FbFixture{db: db, svc: NewService(svcCtx)}
}

func v9FbTableOf(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	if tx.Statement.Schema != nil {
		return tx.Statement.Schema.Table
	}
	return ""
}

// v9FbFailQueryOn makes queries touching table fail from the from-th hit (1-based).
func v9FbFailQueryOn(db *gorm.DB, table string, from int) {
	var hits int
	db.Callback().Query().Before("gorm:query").Register("v9_fb_fail_query_"+table, func(tx *gorm.DB) {
		if v9FbTableOf(tx) != table {
			return
		}
		hits++
		if hits >= from {
			tx.AddError(errors.New("v9 forced query error on " + table))
		}
	})
}

func v9FbFailCreateOn(db *gorm.DB, table string) {
	db.Callback().Create().Before("gorm:create").Register("v9_fb_fail_create_"+table, func(tx *gorm.DB) {
		if v9FbTableOf(tx) == table {
			tx.AddError(errors.New("v9 forced create error on " + table))
		}
	})
}

// v9FbFailUpdateOn fails updates on table; when destKey != "" only updates
// whose destination map contains that key fail.
func v9FbFailUpdateOn(db *gorm.DB, table string) {
	db.Callback().Update().Before("gorm:update").Register("v9_fb_fail_update_"+table, func(tx *gorm.DB) {
		if v9FbTableOf(tx) == table {
			tx.AddError(errors.New("v9 forced update error on " + table))
		}
	})
}

func (f *v9FbFixture) seedV9(t *testing.T, mutate func(fb *model.Feedback)) *model.Feedback {
	t.Helper()
	fb := &model.Feedback{
		PlayerID: "p-1", Contact: "mail@example.com", Content: "content line",
		Category: "bug", Status: dbenum.FeedbackStatusOpen, GameID: "g1", Env: "prod",
	}
	if mutate != nil {
		mutate(fb)
	}
	require.NoError(t, f.db.Create(fb).Error)
	return fb
}

func v9FbGinCtx(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c, w
}

// ---- ConvertToTicket error paths ----

func TestV9ConvertParseIDErrorV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	_, err := f.svc.ConvertToTicket(context.Background(), &ConvertRequest{ID: "abc"})
	require.Error(t, err)
}

func TestV9ConvertTriagedWithoutMarkerV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	rel := f.seedV9(t, func(fb *model.Feedback) {
		fb.Status = dbenum.FeedbackStatusTriaged
		fb.Reply = "已人工处理，无需转单"
	})

	resp, err := f.svc.ConvertToTicket(context.Background(), &ConvertRequest{ID: fmt.Sprint(rel.ID)})
	require.NoError(t, err)
	assert.False(t, resp.AlreadyConverted)

	// New ticket exists and reply was overwritten with the bare marker.
	var tickets int64
	require.NoError(t, f.db.Model(&model.Ticket{}).Count(&tickets).Error)
	assert.Equal(t, int64(1), tickets)
	var after model.Feedback
	require.NoError(t, f.db.First(&after, rel.ID).Error)
	assert.Contains(t, after.Reply, "[已转工单 #")
	assert.Equal(t, dbenum.FeedbackStatusTriaged, after.Status)
}

func TestV9ConvertTicketCreateErrorV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	rel := f.seedV9(t, nil)
	v9FbFailCreateOn(f.db, "tickets")

	_, err := f.svc.ConvertToTicket(context.Background(), &ConvertRequest{ID: fmt.Sprint(rel.ID)})
	require.Error(t, err)
}

func TestV9ConvertFeedbackUpdateErrorV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	rel := f.seedV9(t, nil)
	v9FbFailUpdateOn(f.db, "feedbacks")

	_, err := f.svc.ConvertToTicket(context.Background(), &ConvertRequest{ID: fmt.Sprint(rel.ID)})
	require.Error(t, err)
}

func TestV9ConvertDefaultsAndExtraV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	long := strings.Repeat("很", 60) // no newline, > 40 runes
	rel := f.seedV9(t, func(fb *model.Feedback) {
		fb.PlayerID = ""
		fb.Category = ""
		fb.Content = long
		fb.Attach = "http://att/x.png"
		fb.Rating = 0
	})

	resp, err := f.svc.ConvertToTicket(context.Background(), &ConvertRequest{
		ID:       fmt.Sprint(rel.ID),
		Title:    "   ",
		Priority: "  HIGH  ",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.TicketID)

	var ticket model.Ticket
	require.NoError(t, f.db.First(&ticket).Error)
	assert.Equal(t, "high", ticket.Priority)
	assert.Equal(t, "feedback", ticket.Category)
	assert.Contains(t, ticket.Title, "[反馈#")
	assert.Contains(t, ticket.Title, "…")
	assert.NotContains(t, ticket.Content, "评分")
	assert.NotContains(t, ticket.Content, "【处理备注】")
	assert.Contains(t, string(ticket.Extra), "feedbackAttachment")
	assert.NotContains(t, string(ticket.Extra), "playerId")
}

// ---- pure helpers ----

func TestV9ConvertHelpersV9(t *testing.T) {
	assert.Equal(t, "", extractConvertedTicketID("普通回复"))
	assert.Equal(t, "", extractConvertedTicketID("[已转工单 #12"))
	assert.Equal(t, "12", extractConvertedTicketID("[已转工单 #12]"))
	assert.Equal(t, "9", extractConvertedTicketID("  [已转工单 #9] note"))

	assert.Equal(t, "first", firstLine("first\nsecond"))
	assert.Equal(t, "only", firstLine("only"))

	assert.Equal(t, "short", truncateRunes("short", 10))
	truncated := truncateRunes(strings.Repeat("字", 50), 10)
	assert.Equal(t, strings.Repeat("字", 10)+"…", truncated)

	assert.Equal(t, " b ", firstNonEmpty("", " b ", "c"))
	assert.Equal(t, "", firstNonEmpty("  ", ""))
}

// ---- service error paths ----

func TestV9ServiceListQueryErrorV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	v9FbFailQueryOn(f.db, "feedbacks", 1)

	_, err := f.svc.List(context.Background(), &FeedbackListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestV9ServiceListInvalidStatusFilterV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	f.seedV9(t, nil)

	resp, err := f.svc.List(context.Background(), &FeedbackListRequest{
		Page: 1, PageSize: 10, Status: "bogus", ExcludeStatus: "nope",
	})
	require.NoError(t, err)
	// Unknown filters map to -1 which the model treats as "no filter".
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, int64(1), resp.Total)
}

func TestV9ServiceCreateModelErrorV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	v9FbFailCreateOn(f.db, "feedbacks")

	_, err := f.svc.Create(context.Background(), &FeedbackCreateRequest{
		Contact: "a@b.c", Content: "hello", Category: "bug",
	})
	require.Error(t, err)
}

func TestV9ServiceUpdateErrorBranchesV9(t *testing.T) {
	t.Run("invalid status", func(t *testing.T) {
		f := newFeedbackV9Fixture(t)
		rel := f.seedV9(t, nil)
		_, err := f.svc.Update(context.Background(), &FeedbackUpdateRequest{ID: fmt.Sprint(rel.ID), Status: "bogus"})
		require.ErrorContains(t, err, "反馈状态无效")
	})

	t.Run("find error", func(t *testing.T) {
		f := newFeedbackV9Fixture(t)
		rel := f.seedV9(t, nil)
		v9FbFailQueryOn(f.db, "feedbacks", 1)
		_, err := f.svc.Update(context.Background(), &FeedbackUpdateRequest{ID: fmt.Sprint(rel.ID), Reply: "r"})
		require.Error(t, err)
	})

	t.Run("scope forbidden", func(t *testing.T) {
		f := newFeedbackV9Fixture(t)
		rel := f.seedV9(t, nil)
		ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
		_, err := f.svc.Update(ctx, &FeedbackUpdateRequest{ID: fmt.Sprint(rel.ID), Reply: "r"})
		require.Error(t, err)
	})

	t.Run("update model error", func(t *testing.T) {
		f := newFeedbackV9Fixture(t)
		rel := f.seedV9(t, nil)
		v9FbFailUpdateOn(f.db, "feedbacks")
		_, err := f.svc.Update(context.Background(), &FeedbackUpdateRequest{ID: fmt.Sprint(rel.ID), Reply: "r"})
		require.Error(t, err)
	})

	t.Run("refetch error", func(t *testing.T) {
		f := newFeedbackV9Fixture(t)
		rel := f.seedV9(t, nil)
		v9FbFailQueryOn(f.db, "feedbacks", 2)
		_, err := f.svc.Update(context.Background(), &FeedbackUpdateRequest{ID: fmt.Sprint(rel.ID), Reply: "r"})
		require.Error(t, err)
	})
}

func TestV9ServiceDeleteScopeForbiddenV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	rel := f.seedV9(t, nil)
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})

	err := f.svc.Delete(ctx, &FeedbackDeleteRequest{ID: fmt.Sprint(rel.ID)})
	require.Error(t, err)
}

func TestV9ServiceStatsQueryErrorV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	v9FbFailQueryOn(f.db, "feedbacks", 1)

	_, err := f.svc.Stats(context.Background(), &FeedbackStatsRequest{Days: 7})
	require.Error(t, err)
}

// ---- handler error branches ----

func TestV9HandlerBindErrorsV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	h := NewHandler(f.svc)

	c, w := v9FbGinCtx(http.MethodPost, "/feedback/1", `{invalid`)
	h.Update(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = v9FbGinCtx(http.MethodPost, "/feedback/1/convert", `{invalid`)
	h.ConvertToTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = v9FbGinCtx(http.MethodGet, "/feedback/stats?days=abc", "")
	h.Stats(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestV9HandlerServiceErrorsV9(t *testing.T) {
	f := newFeedbackV9Fixture(t)
	h := NewHandler(f.svc)

	// List via handler with broken model layer.
	v9FbFailQueryOn(f.db, "feedbacks", 1)
	c, w := v9FbGinCtx(http.MethodGet, "/feedback?page=1&pageSize=10", "")
	h.List(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Delete nonexistent id → FindByID error surfaces.
	c, w = v9FbGinCtx(http.MethodDelete, "/feedback/9999", "")
	c.Params = gin.Params{{Key: "id", Value: "9999"}}
	h.Delete(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Stats with nil model.
	nilSvc := NewService(&svc.ServiceContext{})
	nilH := NewHandler(nilSvc)
	c, w = v9FbGinCtx(http.MethodGet, "/feedback/stats", "")
	nilH.Stats(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
