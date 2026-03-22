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
	assert.GreaterOrEqual(t, len(resp.Items), 3)
	assert.GreaterOrEqual(t, resp.Total, 3)
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
	assert.GreaterOrEqual(t, len(resp.Items), 2)

	for _, item := range resp.Items {
		assert.Equal(t, "create", item.Action)
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
	assert.GreaterOrEqual(t, len(resp.Items), 2)

	for _, item := range resp.Items {
		assert.Equal(t, "user1", item.UserID)
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
	assert.GreaterOrEqual(t, len(resp.Items), 1)

	if len(resp.Items) > 0 {
		assert.Equal(t, "user1", resp.Items[0].UserID)
		assert.Equal(t, "create", resp.Items[0].Action)
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
	assert.Len(t, resp.Items, 0)
	assert.Equal(t, 0, resp.Total)
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
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
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
	assert.Equal(t, 1, resp.Page)
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
	assert.Equal(t, 20, resp.PageSize)
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
	assert.Equal(t, 1000, resp.PageSize)
	assert.Len(t, resp.Items, 1000)
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
	assert.Equal(t, 1000000000, resp.Page)
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
	assert.Len(t, resp1.Items, 10)
	assert.Equal(t, 25, resp1.Total)

	resp2, _ := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     2,
		PageSize: 10,
	})
	assert.Len(t, resp2.Items, 10)

	resp3, _ := service.GetAuditLogs(context.Background(), &AuditRequest{
		Page:     3,
		PageSize: 10,
	})
	assert.Len(t, resp3.Items, 5)
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
	assert.Len(t, resp.Items, 0)
	assert.Equal(t, 5, resp.Total)
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
	assert.Equal(t, 5, resp.PageSize)
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
	assert.Equal(t, 10, resp.PageSize)
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
	assert.GreaterOrEqual(t, len(resp.Items), 1)
	assert.NotNil(t, resp.Items[0].Metadata)
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

	// Items should be sorted by CreatedAt descending (newest first)
	if len(resp.Items) >= 3 {
		assert.Equal(t, "user3", resp.Items[0].UserID)
		assert.Equal(t, "user2", resp.Items[1].UserID)
		assert.Equal(t, "user1", resp.Items[2].UserID)
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
	assert.GreaterOrEqual(t, len(resp.Items), 1)

	item := resp.Items[0]
	assert.Equal(t, "testuser", item.UserID)
	assert.Equal(t, "testgame", item.GameID)
	assert.Equal(t, "testenv", item.Env)
	assert.Equal(t, "testaction", item.Action)
	assert.Equal(t, "testtarget", item.Target)
	assert.Equal(t, "testresult", item.Result)
	assert.Equal(t, "testtrace", item.TraceID)
	assert.NotEmpty(t, item.CreatedAt)
	assert.NotNil(t, item.Metadata)
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
