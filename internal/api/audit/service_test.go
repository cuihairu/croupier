package audit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
)

// Singleton test store with proper cleanup
var (
	testAuditStore     *svc.OpsStateStore
	testAuditStoreOnce sync.Once
	testAuditStoreMu   sync.Mutex
)

// setupTestAuditStore creates or resets a shared test store
func setupTestAuditStore(t *testing.T) *svc.OpsStateStore {
	testAuditStoreMu.Lock()
	defer testAuditStoreMu.Unlock()

	testAuditStoreOnce.Do(func() {
		// Create a store with a unique path for testing
		testAuditStore = svc.NewOpsStateStore("")
	})

	// Clear audit entries before each test
	testAuditStore.Update(func(state *svc.OpsState) {
		state.Audit.Entries = nil
		state.Audit.UpdatedAt = time.Now()
	})

	return testAuditStore
}

func TestService_GetAuditLogs_Success(t *testing.T) {
	store := setupTestAuditStore(t)

	// Add some test entries
	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{ID: "trace1", Action: "action1", UserID: "user1", GameID: "game1", Env: "prod", Target: "target1", Result: "success", TraceID: "trace1", CreatedAt: time.Now()},
			svc.OpsAuditEntry{ID: "trace2", Action: "action2", UserID: "user2", GameID: "game2", Env: "dev", Target: "target2", Result: "success", TraceID: "trace2", CreatedAt: time.Now()},
			svc.OpsAuditEntry{ID: "trace3", Action: "action3", UserID: "user1", GameID: "game1", Env: "prod", Target: "target3", Result: "failure", TraceID: "trace3", CreatedAt: time.Now()},
		)
		state.Audit.UpdatedAt = time.Now()
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "OK", resp.Message)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data)

	items, ok := data["items"].([]map[string]interface{})
	assert.True(t, ok)
	assert.GreaterOrEqual(t, len(items), 3)

	total, ok := data["total"].(int)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, total, 3)
}

func TestService_GetAuditLogs_WithActionFilter(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{ID: "trace1", Action: "create", UserID: "user1", GameID: "game1", Env: "prod", Target: "target1", Result: "success", TraceID: "trace1", CreatedAt: time.Now()},
			svc.OpsAuditEntry{ID: "trace2", Action: "delete", UserID: "user2", GameID: "game2", Env: "dev", Target: "target2", Result: "success", TraceID: "trace2", CreatedAt: time.Now()},
			svc.OpsAuditEntry{ID: "trace3", Action: "create", UserID: "user1", GameID: "game1", Env: "prod", Target: "target3", Result: "success", TraceID: "trace3", CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		Action:   "create",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	assert.GreaterOrEqual(t, len(items), 2)

	for _, item := range items {
		assert.Equal(t, "create", item["action"])
	}
}

func TestService_GetAuditLogs_WithUserFilter(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{ID: "trace1", Action: "action1", UserID: "user1", GameID: "game1", Env: "prod", Target: "target1", Result: "success", TraceID: "trace1", CreatedAt: time.Now()},
			svc.OpsAuditEntry{ID: "trace2", Action: "action2", UserID: "user2", GameID: "game2", Env: "dev", Target: "target2", Result: "success", TraceID: "trace2", CreatedAt: time.Now()},
			svc.OpsAuditEntry{ID: "trace3", Action: "action3", UserID: "user1", GameID: "game1", Env: "prod", Target: "target3", Result: "success", TraceID: "trace3", CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		UserID:   "user1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	assert.GreaterOrEqual(t, len(items), 2)

	for _, item := range items {
		assert.Equal(t, "user1", item["userId"])
	}
}

func TestService_GetAuditLogs_WithBothFilters(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{ID: "trace1", Action: "create", UserID: "user1", GameID: "game1", Env: "prod", Target: "target1", Result: "success", TraceID: "trace1", CreatedAt: time.Now()},
			svc.OpsAuditEntry{ID: "trace2", Action: "create", UserID: "user2", GameID: "game2", Env: "dev", Target: "target2", Result: "success", TraceID: "trace2", CreatedAt: time.Now()},
			svc.OpsAuditEntry{ID: "trace3", Action: "delete", UserID: "user1", GameID: "game1", Env: "prod", Target: "target3", Result: "success", TraceID: "trace3", CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		Action:   "create",
		UserID:   "user1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	assert.GreaterOrEqual(t, len(items), 1)

	if len(items) > 0 {
		assert.Equal(t, "user1", items[0]["userId"])
		assert.Equal(t, "create", items[0]["action"])
	}
}

func TestService_GetAuditLogs_EmptyResults(t *testing.T) {
	store := setupTestAuditStore(t)

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		Action:   "nonexistent",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	assert.Len(t, items, 0)
	assert.Equal(t, 0, data["total"])
}

func TestService_GetAuditLogs_DefaultPagination(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		for i := 0; i < 5; i++ {
			state.Audit.Entries = append(state.Audit.Entries,
				svc.OpsAuditEntry{ID: "trace", Action: "action", UserID: "user", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace", CreatedAt: time.Now()},
			)
		}
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 1, data["page"])
	assert.Equal(t, 20, data["size"])
}

func TestService_GetAuditLogs_ZeroPage(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{Action: "action", UserID: "user", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace", CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     0,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 1, data["page"])
}

func TestService_GetAuditLogs_ZeroPageSize(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{Action: "action", UserID: "user", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace", CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 0,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 20, data["size"])
}

func TestService_GetAuditLogs_MaxPageSize(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		for i := 0; i < 2000; i++ {
			state.Audit.Entries = append(state.Audit.Entries,
				svc.OpsAuditEntry{Action: "action", UserID: "user", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace", CreatedAt: time.Now()},
			)
		}
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10000,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 1000, data["size"])

	items := data["items"].([]map[string]interface{})
	assert.Len(t, items, 1000)
}

func TestService_GetAuditLogs_MaxPage(t *testing.T) {
	store := setupTestAuditStore(t)

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     2_000_000_001,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	// The page should be capped at 1_000_000_000
	pageVal := data["page"]
	assert.Equal(t, 1000000000, pageVal)
}

func TestService_GetAuditLogs_Pagination(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		for i := 0; i < 25; i++ {
			state.Audit.Entries = append(state.Audit.Entries,
				svc.OpsAuditEntry{Action: "action", UserID: "user", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace", CreatedAt: time.Now()},
			)
		}
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp1, _ := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})
	data1 := resp1.Data.(map[string]interface{})
	items1 := data1["items"].([]map[string]interface{})
	assert.Len(t, items1, 10)
	assert.Equal(t, 25, data1["total"])

	resp2, _ := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     2,
		PageSize: 10,
	})
	data2 := resp2.Data.(map[string]interface{})
	items2 := data2["items"].([]map[string]interface{})
	assert.Len(t, items2, 10)

	resp3, _ := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     3,
		PageSize: 10,
	})
	data3 := resp3.Data.(map[string]interface{})
	items3 := data3["items"].([]map[string]interface{})
	assert.Len(t, items3, 5)
}

func TestService_GetAuditLogs_PageBeyondData(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		for i := 0; i < 5; i++ {
			state.Audit.Entries = append(state.Audit.Entries,
				svc.OpsAuditEntry{Action: "action", UserID: "user", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace", CreatedAt: time.Now()},
			)
		}
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     10,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	assert.Len(t, items, 0)
	assert.Equal(t, 5, data["total"])
}

func TestService_GetAuditLogs_NilRequest(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{Action: "action", UserID: "user", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace", CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_GetAuditLogs_NilOpsStateStore(t *testing.T) {
	service := NewService(&svc.ServiceContext{
		OpsStateStore: nil,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_GetAuditLogs_WithSizeAlias(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{Action: "action", UserID: "user", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace", CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page: 1,
		Size: 5,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 5, data["size"])
}

func TestService_GetAuditLogs_PageSizeTakesPrecedence(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{Action: "action", UserID: "user", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace", CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
		Size:     5,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 10, data["size"])
}

func TestService_GetAuditLogs_WithMetadata(t *testing.T) {
	store := setupTestAuditStore(t)

	metadata := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}
	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{ID: "trace1", Action: "action1", UserID: "user1", GameID: "game1", Env: "prod", Target: "target1", Result: "success", TraceID: "trace1", Metadata: metadata, CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	assert.GreaterOrEqual(t, len(items), 1)

	itemMetadata := items[0]["metadata"]
	assert.NotNil(t, itemMetadata)
}

func TestService_GetAuditLogs_SortedByCreatedAt(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{ID: "trace1", Action: "action1", UserID: "user1", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace1", CreatedAt: time.Now().Add(-2 * time.Hour)},
			svc.OpsAuditEntry{ID: "trace2", Action: "action2", UserID: "user2", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace2", CreatedAt: time.Now().Add(-1 * time.Hour)},
			svc.OpsAuditEntry{ID: "trace3", Action: "action3", UserID: "user3", GameID: "game", Env: "prod", Target: "target", Result: "success", TraceID: "trace3", CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})

	// Items should be sorted by CreatedAt descending (newest first)
	if len(items) >= 3 {
		assert.Equal(t, "user3", items[0]["userId"])
		assert.Equal(t, "user2", items[1]["userId"])
		assert.Equal(t, "user1", items[2]["userId"])
	}
}

func TestService_GetAuditLogs_AllFieldsPresent(t *testing.T) {
	store := setupTestAuditStore(t)

	store.Update(func(state *svc.OpsState) {
		state.Audit.Entries = append(state.Audit.Entries,
			svc.OpsAuditEntry{ID: "trace1", Action: "testaction", UserID: "testuser", GameID: "testgame", Env: "testenv", Target: "testtarget", Result: "testresult", TraceID: "testtrace", Metadata: map[string]interface{}{"metaKey": "metaValue"}, CreatedAt: time.Now()},
		)
	})

	service := NewService(&svc.ServiceContext{
		OpsStateStore: store,
	})

	resp, err := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]map[string]interface{})
	assert.GreaterOrEqual(t, len(items), 1)

	item := items[0]
	assert.Equal(t, "testuser", item["userId"])
	assert.Equal(t, "testgame", item["gameId"])
	assert.Equal(t, "testenv", item["env"])
	assert.Equal(t, "testaction", item["action"])
	assert.Equal(t, "testtarget", item["target"])
	assert.Equal(t, "testresult", item["result"])
	assert.Equal(t, "testtrace", item["traceId"])
	assert.NotEmpty(t, item["createdAt"])
	assert.NotNil(t, item["metadata"])
}

func TestNewService(t *testing.T) {
	store := setupTestAuditStore(t)
	svcCtx := &svc.ServiceContext{
		OpsStateStore: store,
	}

	service := NewService(svcCtx)

	assert.NotNil(t, service)
	assert.Equal(t, svcCtx, service.svcCtx)
}
