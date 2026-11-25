package schedule

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	transportcore "github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// stubSessionCaller implements transport.SessionCaller: every call returns the
// canned response.
type stubSessionCaller struct {
	respBody []byte
}

func (s *stubSessionCaller) Call(_ context.Context, msgID uint32, _ []byte) (uint32, []byte, error) {
	return msgID, s.respBody, nil
}

// stubSessionResolver resolves any agent to the same caller.
type stubSessionResolver struct {
	caller transportcore.SessionCaller
}

func (r *stubSessionResolver) ResolveAgentConn(_ string) (transportcore.SessionCaller, bool) {
	return r.caller, true
}

// newTestEnvWithDispatcher is the same as newTestEnv but allows injecting a custom
// Dispatcher.
func newTestEnvWithDispatcher(t *testing.T, d *dispatch.Dispatcher) (*gin.Engine, *gorm.DB) {
	t.Helper()
	name := fmt.Sprintf("schedule_trig_%d", atomic.AddUint64(&dbSeq, 1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	svcCtx := &svc.ServiceContext{
		TaskScheduleModel: model.NewTaskScheduleModel(db),
		AdminModel:        model.NewAdminModel(db),
		// ARCH-024 (R30-01): Govern now runs the explicit function ACL in
		// CheckInvokePermission, and a nil FunctionModel fails closed (403). Inject a
		// FunctionModel as the task service tests do so TriggerNow can reach the
		// dispatch path.
		FunctionModel: model.NewFunctionModel(db),
		Dispatcher:    d,
	}
	// TriggerNow loads the caller principal through the ARCH-024 Govern chain (username=tester).
	seedScheduleAdmin(t, db, "tester")
	h := NewHandler(NewService(svcCtx))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "tester")
		ctx = svc.WithGameScope(ctx, svc.GameScope{GameID: "demo", Env: "prod"})
		c.Request = c.Request.WithContext(ctx)
	})
	g := r.Group("/api/v1/schedules")
	g.POST("", h.Create)
	g.POST("/:id/trigger", h.TriggerNow)
	return r, db
}

// TriggerNow success path: the dispatcher hits an agent carrying a fake session,
// and the handler should answer 200 with the taskRunId returned by the agent.
func TestHandler_TriggerNow_Success(t *testing.T) {
	store := reg.NewStore()
	d := dispatch.NewDispatcher(store)
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-trigger",
		GameID:   "demo",
		Env:      "prod",
		ExpireAt: time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{
			"fn.reset": {Enabled: true},
		},
	}))

	respBody, err := proto.Marshal(&sdkv1.StartTaskResponse{TaskId: "task-trigger-1"})
	require.NoError(t, err)
	d.SetSessionResolver(&stubSessionResolver{caller: &stubSessionCaller{respBody: respBody}})

	r, _ := newTestEnvWithDispatcher(t, d)
	id := mustCreate(t, r)
	rec := doReq(r, http.MethodPost, "/api/v1/schedules/"+strconv.FormatUint(uint64(id), 10)+"/trigger")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "task-trigger-1")
}
