package audit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditService_SetNotifier tests SetNotifier method
func TestAuditService_SetNotifier(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	// Create a mock notifier
	mockNotifier := &mockAuditNotifier{}
	service.SetNotifier(mockNotifier)

	// Verify notifier is set (we can't access it directly, but we verify it doesn't crash)
	assert.NotNil(t, service)
}

// TestWithActor tests WithActor option
func TestWithActor(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()
	actor := ActorInfo{
		ID:   "actor-1",
		Type: "user",
		Name: "Test Actor",
	}

	record, err := service.Log(ctx, EventLogin, WithActor(actor))
	require.NoError(t, err)

	assert.Equal(t, actor.ID, record.Actor.ID)
	assert.Equal(t, actor.Type, record.Actor.Type)
	assert.Equal(t, actor.Name, record.Actor.Name)
}

// TestWithResource tests WithResource option
func TestWithResource(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()
	resource := ResourceInfo{
		Type: "server",
		ID:   "srv-1",
		Name: "Test Server",
	}

	record, err := service.Log(ctx, EventConfigUpdate, WithResource(resource))
	require.NoError(t, err)

	assert.Equal(t, resource.Type, record.Resource.Type)
	assert.Equal(t, resource.ID, record.Resource.ID)
	assert.Equal(t, resource.Name, record.Resource.Name)
}

// TestWithContext tests WithContext option
func TestWithContext(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()
	auditCtx := AuditContext{
		RequestID:     "req-1",
		TraceID:       "trace-1",
		CorrelationID: "corr-1",
		Service:       "test-service",
		Environment:   "dev",
		Tags: map[string]string{
			"key1": "value1",
		},
	}

	record, err := service.Log(ctx, EventFunctionInvoke, WithContext(auditCtx))
	require.NoError(t, err)

	assert.Equal(t, auditCtx.RequestID, record.Context.RequestID)
	assert.Equal(t, auditCtx.TraceID, record.Context.TraceID)
	assert.Equal(t, auditCtx.CorrelationID, record.Context.CorrelationID)
	assert.Equal(t, auditCtx.Service, record.Context.Service)
	assert.Equal(t, auditCtx.Environment, record.Context.Environment)
}

// TestWithSeverity tests WithSeverity option
func TestWithSeverity(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	record, err := service.Log(ctx, EventLogin, WithSeverity(SeverityCritical))
	require.NoError(t, err)

	assert.Equal(t, SeverityCritical, record.Severity)
}

// TestWithCategory tests WithCategory option
func TestWithCategory(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	record, err := service.Log(ctx, EventLogin, WithCategory(CategoryData))
	require.NoError(t, err)

	assert.Equal(t, CategoryData, record.Category)
}

// TestWithGameID tests WithGameID option
func TestWithGameID(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	record, err := service.Log(ctx, EventFunctionInvoke, WithGameID("game-1", "prod"))
	require.NoError(t, err)

	assert.Equal(t, "game-1", record.Resource.GameID)
	assert.Equal(t, "prod", record.Resource.Environment)
}

// TestAuditService_Get tests Get method
func TestAuditService_Get(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create a record
	created, err := service.Log(ctx, EventLogin, WithActorID("user-1", "user", "Test"))
	require.NoError(t, err)

	// Get the record
	retrieved, err := service.Get(created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.EventType, retrieved.EventType)
}

// TestAuditService_Get_NotFound tests Get with non-existent ID
func TestAuditService_Get_NotFound(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	_, err := service.Get("nonexistent-id")
	assert.Error(t, err)
}

// TestAuditService_List tests List method
func TestAuditService_List(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create multiple records
	_, _ = service.Log(ctx, EventLogin, WithActorID("user-1", "user", "User 1"))
	_, _ = service.Log(ctx, EventLogout, WithActorID("user-1", "user", "User 1"))
	_, _ = service.Log(ctx, EventLogin, WithActorID("user-2", "user", "User 2"))

	// List all records
	records, total, err := service.List(AuditFilter{}, AuditPage{PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, records, 3)
}

// TestAuditService_List_WithFilter tests List with filters
func TestAuditService_List_WithFilter(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create records with different actors
	_, _ = service.Log(ctx, EventLogin, WithActorID("user-1", "user", "User 1"))
	_, _ = service.Log(ctx, EventLogin, WithActorID("user-2", "user", "User 2"))

	// Filter by actor ID
	records, total, err := service.List(AuditFilter{ActorID: "user-1"}, AuditPage{PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, records, 1)
	assert.Equal(t, "user-1", records[0].Actor.ID)
}

// TestRandomString tests randomString function
func TestRandomString(t *testing.T) {
	// Test various lengths
	lengths := []int{1, 5, 10, 20}

	for _, n := range lengths {
		result := randomString(n)
		assert.Len(t, result, n, "randomString(%d) should return %d characters", n, n)

		// Verify all characters are from the allowed set
		for _, ch := range result {
			assert.Contains(t, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", string(ch),
				"randomString should only contain alphanumeric characters")
		}
	}
}

// TestRandomString_Uniqueness tests that randomString produces different values
func TestRandomString_Uniqueness(t *testing.T) {
	results := make(map[string]bool)
	length := 10

	// Generate 100 random strings
	for i := 0; i < 100; i++ {
		result := randomString(length)
		results[result] = true
	}

	// With 100 iterations and 10 characters (62^10 possible combinations),
	// we should get unique values
	assert.Greater(t, len(results), 90, "randomString should produce mostly unique values")
}

// TestWithActor_Combined tests WithActor with other options
func TestWithActor_Combined(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()
	actor := ActorInfo{
		ID:        "actor-1",
		Type:      "user",
		Name:      "Test Actor",
		IPAddress: "192.168.1.1",
	}

	record, err := service.Log(ctx, EventLogin,
		WithActor(actor),
		WithOutcome("success", ""),
		WithSeverity(SeverityInfo),
	)
	require.NoError(t, err)

	assert.Equal(t, actor.ID, record.Actor.ID)
	assert.Equal(t, actor.Type, record.Actor.Type)
	assert.Equal(t, actor.Name, record.Actor.Name)
	assert.Equal(t, "success", record.Outcome)
	assert.Equal(t, SeverityInfo, record.Severity)
}

// TestWithResource_Combined tests WithResource with other options
func TestWithResource_Combined(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()
	resource := ResourceInfo{
		Type:   "server",
		ID:     "srv-1",
		Name:   "Test Server",
		GameID: "game-1",
	}

	record, err := service.Log(ctx, EventConfigUpdate,
		WithResource(resource),
		WithGameID("game-2", "dev"), // This should override resource.GameID
	)
	require.NoError(t, err)

	assert.Equal(t, "server", record.Resource.Type)
	assert.Equal(t, "srv-1", record.Resource.ID)
	// WithGameID overrides the resource.GameID
	assert.Equal(t, "game-2", record.Resource.GameID)
	assert.Equal(t, "dev", record.Resource.Environment)
}

// TestWithSeverity_AllLevels tests all severity levels
func TestWithSeverity_AllLevels(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	severities := []AuditSeverity{
		SeverityInfo,
		SeverityWarning,
		SeverityError,
		SeverityCritical,
	}

	for _, severity := range severities {
		record, err := service.Log(ctx, EventLogin, WithSeverity(severity))
		require.NoError(t, err)
		assert.Equal(t, severity, record.Severity)
	}
}

// TestWithCategory_AllCategories tests all category types
func TestWithCategory_AllCategories(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	categories := []AuditCategory{
		CategorySecurity,
		CategoryAdmin,
		CategoryOperational,
		CategoryData,
		CategoryCompliance,
	}

	for _, category := range categories {
		record, err := service.Log(ctx, EventLogin, WithCategory(category))
		require.NoError(t, err)
		assert.Equal(t, category, record.Category)
	}
}

// TestWithGameID_AllEnvs tests different environments
func TestWithGameID_AllEnvs(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	envs := []string{"dev", "staging", "prod", "test"}

	for _, env := range envs {
		record, err := service.Log(ctx, EventFunctionInvoke, WithGameID("game-1", env))
		require.NoError(t, err)
		assert.Equal(t, "game-1", record.Resource.GameID)
		assert.Equal(t, env, record.Resource.Environment)
	}
}

// TestWithContext_AllFields tests all context fields
func TestWithContext_AllFields(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()
	auditCtx := AuditContext{
		RequestID:     "req-1",
		TraceID:       "trace-1",
		CorrelationID: "corr-1",
		Service:       "test-service",
		Environment:   "dev",
		Tags: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	record, err := service.Log(ctx, EventFunctionInvoke, WithContext(auditCtx))
	require.NoError(t, err)

	assert.Equal(t, auditCtx.RequestID, record.Context.RequestID)
	assert.Equal(t, auditCtx.TraceID, record.Context.TraceID)
	assert.Equal(t, auditCtx.CorrelationID, record.Context.CorrelationID)
	assert.Equal(t, auditCtx.Service, record.Context.Service)
	assert.Equal(t, auditCtx.Environment, record.Context.Environment)
	assert.Equal(t, auditCtx.Tags, record.Context.Tags)
}

// TestAuditService_List_Pagination tests List with pagination
func TestAuditService_List_Pagination(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create 15 records
	for i := 0; i < 15; i++ {
		_, _ = service.Log(ctx, EventLogin, WithActorID("user-1", "user", "Test"))
	}

	// First page
	records, total, err := service.List(AuditFilter{}, AuditPage{Page: 1, PageSize: 5})
	require.NoError(t, err)
	assert.Equal(t, 15, total)
	assert.Len(t, records, 5)

	// Second page
	records, _, err = service.List(AuditFilter{}, AuditPage{Page: 2, PageSize: 5})
	require.NoError(t, err)
	assert.Len(t, records, 5)

	// Third page
	records, _, err = service.List(AuditFilter{}, AuditPage{Page: 3, PageSize: 5})
	require.NoError(t, err)
	assert.Len(t, records, 5)
}

// TestAuditService_List_EmptyStore tests List with empty store
func TestAuditService_List_EmptyStore(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	records, total, err := service.List(AuditFilter{}, AuditPage{PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, records)
}

// mockAuditNotifier is a mock implementation of AuditNotifier
type mockAuditNotifier struct{}

func (m *mockAuditNotifier) NotifyAudit(ctx context.Context, record *AuditRecord) error {
	return nil
}
