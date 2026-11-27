package task

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/tasks"
)

// AUD-089: Task Events continuation cursor.
//
// The legacy response field NextSeq is MAX(seq)+1 (see
// internal/api/task/service.go Service.Events and internal/model/task_model.go
// TaskEventModel.NextSeq). A caller that feeds NextSeq back as after_seq
// filters with seq > afterSeq, so an event appended between the two polls
// with exactly seq == NextSeq is skipped. The fix adds a `cursor` field whose
// semantics are "last read sequence number": seq > cursor reads strictly
// newer events and can never skip.

// seedTaskEvents inserts the given seqs directly (AppendEvent assigns MAX+1,
// which cannot express "an event appended after the cursor was returned").
func seedTaskEvents(t *testing.T, db *gorm.DB, taskID string, seqs ...int64) {
	t.Helper()
	for _, seq := range seqs {
		require.NoError(t, db.Create(&model.TaskEvent{
			TaskID: taskID, Seq: seq, Type: string(tasks.EventProgress), Progress: 50, Message: "evt",
		}).Error)
	}
}

// TestEvents_CursorRoundTripsNewlyAppendedEvent proves the reported defect is
// fixed: read seq 1, server returns NextSeq 2, and using the new `cursor`
// (1) still reads a seq-2 event appended afterwards, whereas feeding NextSeq
// back as after_seq would skip it. The event is read exactly once.
func TestEvents_CursorRoundTripsNewlyAppendedEvent(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-cursor-1", "player.ban", tasks.StatusRunning)
	seedTaskEvents(t, svcCtx.DB, "t-cursor-1", 1)
	svc := NewService(svcCtx)

	first, err := svc.Events(context.Background(), &EventsRequest{ID: "t-cursor-1"})
	require.NoError(t, err)
	require.Len(t, first.Items, 1)
	assert.Equal(t, int64(1), first.Items[0].Seq)
	assert.Equal(t, int64(2), first.NextSeq, "legacy nextSeq stays MAX(seq)+1")
	assert.Equal(t, int64(1), first.Cursor, "cursor is the last read seq")

	// Event appended after the first response.
	seedTaskEvents(t, svcCtx.DB, "t-cursor-1", 2)

	// Legacy behaviour preserved: feeding NextSeq back skips seq 2.
	legacy, err := svc.Events(context.Background(), &EventsRequest{ID: "t-cursor-1", AfterSeq: first.NextSeq})
	require.NoError(t, err)
	assert.Empty(t, legacy.Items, "legacy nextSeq round-trip skips the appended event (documented, unchanged)")

	// New cursor round-trips: seq 2 is returned exactly once.
	second, err := svc.Events(context.Background(), &EventsRequest{ID: "t-cursor-1", Cursor: &first.Cursor})
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	assert.Equal(t, int64(2), second.Items[0].Seq)
	assert.Equal(t, int64(2), second.Cursor)
}

// TestEvents_CursorNoIncrementPollReturnsEmptyWithUnchangedCursor covers the
// "nothing new" poll: no duplicates and a stable cursor.
func TestEvents_CursorNoIncrementPollReturnsEmptyWithUnchangedCursor(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-cursor-2", "player.ban", tasks.StatusRunning)
	seedTaskEvents(t, svcCtx.DB, "t-cursor-2", 1, 2, 3)
	svc := NewService(svcCtx)

	first, err := svc.Events(context.Background(), &EventsRequest{ID: "t-cursor-2", Cursor: &[]int64{0}[0]})
	require.NoError(t, err)
	require.Len(t, first.Items, 3)
	assert.Equal(t, int64(3), first.Cursor)

	second, err := svc.Events(context.Background(), &EventsRequest{ID: "t-cursor-2", Cursor: &first.Cursor})
	require.NoError(t, err)
	assert.Empty(t, second.Items, "no new events -> empty page")
	assert.Equal(t, first.Cursor, second.Cursor, "cursor unchanged when nothing new arrives")
}

// TestEvents_CursorStartsAtZeroFromFirstEvent covers cursor=0 reading every
// event from the beginning (pointer distinguishes explicit 0 from absent).
func TestEvents_CursorStartsAtZeroFromFirstEvent(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-cursor-3", "player.ban", tasks.StatusRunning)
	seedTaskEvents(t, svcCtx.DB, "t-cursor-3", 7, 8)
	svc := NewService(svcCtx)

	zero := int64(0)
	resp, err := svc.Events(context.Background(), &EventsRequest{ID: "t-cursor-3", Cursor: &zero})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, int64(7), resp.Items[0].Seq)
	assert.Equal(t, int64(8), resp.Items[1].Seq)
	assert.Equal(t, int64(8), resp.Cursor)
}

// TestEvents_CursorHTTPQueryParam binds ?cursor= from the query string and
// round-trips it back, including the empty-poll case.
func TestEvents_CursorHTTPQueryParam(t *testing.T) {
	t.Parallel()
	handler, svc := setupHandler(t)
	run := seedTaskRun(t, svc.svcCtx.DB, "t-cursor-http", "player.ban", tasks.StatusRunning)
	// AppendEvent assigns seq 1 (MAX(seq)+1 on an empty task).
	require.NoError(t, svc.runtime.AppendEvent(context.Background(), run.TaskID, tasks.EventProgress, 10, "halfway", []byte(`{"count":1}`)))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/t-cursor-http/events?cursor=0", nil)
	c.Params = []gin.Param{{Key: "id", Value: "t-cursor-http"}}
	handler.Events(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var response EventsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Items, 1)
	assert.Equal(t, int64(1), response.Items[0].Seq)
	assert.Equal(t, int64(1), response.Cursor)
	assert.Equal(t, int64(2), response.NextSeq, "legacy nextSeq unchanged")

	// No new event: empty page, cursor unchanged.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request, _ = http.NewRequest("GET", "/tasks/t-cursor-http/events?cursor=1", nil)
	c2.Params = []gin.Param{{Key: "id", Value: "t-cursor-http"}}
	handler.Events(c2)
	require.Equal(t, http.StatusOK, w2.Code, w2.Body.String())

	var response2 EventsResponse
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &response2))
	assert.Empty(t, response2.Items)
	assert.Equal(t, int64(1), response2.Cursor)
}

// TestEvents_CursorTakesPrecedenceOverLegacyAfterSeq documents the handler
// contract: when both are supplied, `cursor` (the new round-trippable value)
// wins so callers can never accidentally skip an event.
func TestEvents_CursorTakesPrecedenceOverLegacyAfterSeq(t *testing.T) {
	t.Parallel()
	handler, svc := setupHandler(t)
	run := seedTaskRun(t, svc.svcCtx.DB, "t-cursor-prec", "player.ban", tasks.StatusRunning)
	require.NoError(t, svc.runtime.AppendEvent(context.Background(), run.TaskID, tasks.EventProgress, 10, "halfway", []byte(`{"count":1}`)))
	require.NoError(t, svc.runtime.AppendEvent(context.Background(), run.TaskID, tasks.EventProgress, 20, "later", []byte(`{"count":2}`)))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/t-cursor-prec/events?after_seq=0&cursor=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "t-cursor-prec"}}
	handler.Events(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var response EventsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Items, 1, "cursor=1 must read only seq 2, ignoring after_seq=0")
	assert.Equal(t, int64(2), response.Items[0].Seq)
	assert.Equal(t, int64(2), response.Cursor)
}

// TestEvents_CursorRejectsNegativeQueryParam keeps the query validation
// symmetric with after_seq.
func TestEvents_CursorRejectsNegativeQueryParam(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/t-1/events?cursor=-1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "t-1"}}

	handler.Events(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestEvents_CursorLegacyNextSeqUnchanged pins the compatibility promise: the
// legacy after_seq round-trip still behaves exactly as before the cursor
// field existed (it advances past the last read seq).
func TestEvents_CursorLegacyNextSeqUnchanged(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-cursor-legacy", "player.ban", tasks.StatusSucceeded)
	seedTaskEvents(t, svcCtx.DB, "t-cursor-legacy", 1, 2)
	svc := NewService(svcCtx)

	// Legacy first call with no cursor: identical to pre-AUD-089 behaviour.
	first, err := svc.Events(context.Background(), &EventsRequest{ID: "t-cursor-legacy"})
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	assert.Equal(t, int64(3), first.NextSeq)
	assert.Equal(t, int64(2), first.Cursor)

	// Legacy caller feeding NextSeq back gets an empty page (no duplicates).
	second, err := svc.Events(context.Background(), &EventsRequest{ID: "t-cursor-legacy", AfterSeq: first.NextSeq})
	require.NoError(t, err)
	assert.Empty(t, second.Items)
	assert.Equal(t, int64(3), second.Cursor, "cursor holds the last read seq")
	assert.Equal(t, first.NextSeq, second.NextSeq, "legacy nextSeq semantics unchanged")
}

// TestTaskEventModel_ListByTaskIDStrictlyGreater is the AUD-089 SQLite probe:
// the store filter (seq > afterSeq) is what makes a "last read" cursor safe,
// and it is pinned here so a future refactor cannot silently weaken it.
func TestTaskEventModel_ListByTaskIDStrictlyGreater(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskEvents(t, svcCtx.DB, "t-probe", 1, 2)

	em := model.NewTaskEventModel(svcCtx.DB)
	got, err := em.ListByTaskID(context.Background(), "t-probe", 1)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].Seq)

	// Feeding a "max+1" cursor back reads nothing (the legacy trap).
	got, err = em.ListByTaskID(context.Background(), "t-probe", 2)
	require.NoError(t, err)
	assert.Empty(t, got)
}
