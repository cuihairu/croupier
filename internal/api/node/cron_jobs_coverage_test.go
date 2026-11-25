package node

// Error-path coverage for the #24 aggregation chain (coverage sweep: handler
// ListAllCronJobs at 60%, service at 93.2%). The existing
// cron_jobs_aggregate_test.go covers the aggregation/cache/offline main chain;
// this file adds: uninitialized NodeModel, node table query failure, and the
// handler's error and success branches.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// NodeModel not initialized: the service reports "node table not initialized"
// outright instead of panicking.
func TestListAllCronJobs_NilNodeModelErrors(t *testing.T) {
	svcUnderTest := NewService(&svc.ServiceContext{})
	reports, err := svcUnderTest.ListAllCronJobs(context.Background())
	require.Error(t, err)
	assert.Nil(t, reports)
	assert.Contains(t, err.Error(), "节点表未初始化")
}

// The nodes table is missing (migration never ran / database corrupted): the
// List error is surfaced as-is, neither swallowed nor turned into a panic.
func TestListAllCronJobs_NodeListError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/node-bare.db"), &gorm.Config{})
	require.NoError(t, err)
	// Deliberately skipping AutoMigrate: the nodes table does not exist.
	svcUnderTest := NewService(&svc.ServiceContext{DB: db, NodeModel: model.NewNodeModel(db)})

	reports, err := svcUnderTest.ListAllCronJobs(context.Background())
	require.Error(t, err, "missing nodes table must surface as error")
	assert.Nil(t, reports)
}

// Handler error branch: on service failure a unified error object is returned
// via response.Error.
func TestHandler_ListAllCronJobs_ErrorPath(t *testing.T) {
	handler := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/cron-jobs", "")
	handler.ListAllCronJobs(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotEmpty(t, body["error"], "unified error object expected, got %s", rec.Body.String())
}

// Handler success branch: with nodes present, 200 + items/total (an offline
// node with ok=false is listed too).
func TestHandler_ListAllCronJobs_Success(t *testing.T) {
	svcUnderTest := newAggregateService(t, perNodeResolver{})
	db := svcUnderTest.svcCtx.DB
	require.NoError(t, db.Create(&model.Node{NodeID: "agent-offline", Name: "host-b", Status: "offline"}).Error)

	handler := NewHandler(svcUnderTest)
	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/cron-jobs", "")
	handler.ListAllCronJobs(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Items []NodeCronJobsReport `json:"items"`
		Total int                  `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, 1, body.Total)
	assert.Equal(t, "agent-offline", body.Items[0].NodeID)
	assert.False(t, body.Items[0].OK)
}

// failingCaller always fails its tunnel call (the collection error surfaces in
// the report).
type failingCaller struct{}

func (failingCaller) Call(context.Context, uint32, []byte) (uint32, []byte, error) {
	return 0, nil, errors.New("tunnel down")
}

// Two tiers of error degradation in the aggregated report: an uninitialized
// session table (deps error) and a single-node collection failure (Call error)
// both end up in rep.Error with ok=false, without blocking aggregation of the
// other nodes.
func TestListAllCronJobs_DepsAndCollectErrorsSurfaceInReport(t *testing.T) {
	db := newNodeTestDB(t)

	// Deps error: AgentSessions not injected.
	nilDeps := NewService(&svc.ServiceContext{DB: db, NodeModel: model.NewNodeModel(db)})
	require.NoError(t, db.Create(&model.Node{NodeID: "agent-1", Name: "host-1", Status: "online"}).Error)
	reports, err := nilDeps.ListAllCronJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, reports, 1)
	assert.False(t, reports[0].OK)
	assert.Contains(t, reports[0].Error, "会话表未初始化")

	// Collection error: the resolver is online but Call fails.
	withFail := NewService(&svc.ServiceContext{
		DB:            db,
		NodeModel:     model.NewNodeModel(db),
		AgentSessions: perNodeResolver{callers: map[string]transport.SessionCaller{"agent-1": failingCaller{}}},
	})
	reports, err = withFail.ListAllCronJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, reports, 1)
	assert.False(t, reports[0].OK)
	assert.Contains(t, reports[0].Error, "tunnel down")
}
