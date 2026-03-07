package audit

import (
	"context"
	"sort"
	"testing"
	"time"
)

func TestAuditService_Log(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()
	record, err := service.Log(ctx, EventLogin,
		WithActorID("user-1", "user", "Test User"),
		WithResourceID("session", "sess-123"),
		WithIPAddress("192.168.1.1", "test-agent"),
		WithOutcome("success", ""),
	)

	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	if record.ID == "" {
		t.Error("Record ID should not be empty")
	}

	if record.EventType != EventLogin {
		t.Errorf("Expected event type %s, got %s", EventLogin, record.EventType)
	}

	if record.Actor.ID != "user-1" {
		t.Errorf("Expected actor ID user-1, got %s", record.Actor.ID)
	}

	if record.ChainInfo.Hash == "" {
		t.Error("Chain hash should not be empty")
	}

	if record.ChainInfo.Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", record.ChainInfo.Sequence)
	}
}

func TestAuditService_MultipleRecords(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create multiple records
	for i := 0; i < 5; i++ {
		_, err := service.Log(ctx, EventFunctionInvoke,
			WithActorID("user-1", "user", "Test User"),
			WithResourceID("function", "func-1"),
		)
		if err != nil {
			t.Fatalf("Log %d failed: %v", i, err)
		}
	}

	// Verify chain
	records, total, err := store.List(AuditFilter{}, AuditPage{PageSize: 100})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total != 5 {
		t.Errorf("Expected 5 records, got %d", total)
	}

	// Sort records by sequence for chain verification
	sortedRecords := make([]*AuditRecord, len(records))
	copy(sortedRecords, records)
	sort.Slice(sortedRecords, func(i, j int) bool {
		return sortedRecords[i].ChainInfo.Sequence < sortedRecords[j].ChainInfo.Sequence
	})

	// Verify chain integrity
	for i := 1; i < len(sortedRecords); i++ {
		if sortedRecords[i].ChainInfo.PrevHash != sortedRecords[i-1].ChainInfo.Hash {
			t.Errorf("Chain broken at index %d: prev hash mismatch", i)
		}
		if sortedRecords[i].ChainInfo.Sequence != sortedRecords[i-1].ChainInfo.Sequence+1 {
			t.Errorf("Chain sequence broken at index %d", i)
		}
	}
}

func TestAuditService_WithChanges(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()
	before := map[string]interface{}{
		"name":  "old-name",
		"value": 100,
	}
	after := map[string]interface{}{
		"name":  "new-name",
		"value": 200,
	}

	record, err := service.Log(ctx, EventConfigUpdate,
		WithActorID("admin-1", "user", "Admin"),
		WithResourceID("config", "cfg-1"),
		WithChanges(before, after),
	)

	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	if record.Changes == nil {
		t.Fatal("Changes should not be nil")
	}

	if len(record.Changes.DiffFields) == 0 {
		t.Error("DiffFields should have entries")
	}

	// Check that both name and value are in diff fields
	diffMap := make(map[string]bool)
	for _, f := range record.Changes.DiffFields {
		diffMap[f] = true
	}

	if !diffMap["name"] || !diffMap["value"] {
		t.Error("Expected both name and value to be in diff fields")
	}
}

func TestAuditService_MaskSensitiveData(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()
	details := map[string]interface{}{
		"username": "testuser",
		"password": "secret123",
		"api_key":  "key-12345",
		"data": map[string]interface{}{
			"token": "token-abc",
			"value": 100,
		},
	}

	record, err := service.Log(ctx, EventLogin,
		WithActorID("user-1", "user", "Test"),
		WithDetails(details),
	)

	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Check sensitive fields are masked
	if record.Details["password"] != "***MASKED***" {
		t.Error("Password should be masked")
	}
	if record.Details["api_key"] != "***MASKED***" {
		t.Error("API key should be masked")
	}

	// Check non-sensitive fields are preserved
	if record.Details["username"] != "testuser" {
		t.Error("Username should not be masked")
	}

	// Check nested sensitive fields
	if nested, ok := record.Details["data"].(map[string]interface{}); ok {
		if nested["token"] != "***MASKED***" {
			t.Error("Nested token should be masked")
		}
		if nested["value"] != 100 {
			t.Error("Nested value should not be masked")
		}
	}
}

func TestAuditService_Filter(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create records with different event types
	service.Log(ctx, EventLogin, WithActorID("user-1", "user", "User 1"))
	service.Log(ctx, EventLogout, WithActorID("user-1", "user", "User 1"))
	service.Log(ctx, EventLogin, WithActorID("user-2", "user", "User 2"))
	service.Log(ctx, EventAccessDenied, WithActorID("user-2", "user", "User 2"))

	// Filter by event type
	records, total, err := store.List(AuditFilter{
		EventType: []AuditEventType{EventLogin},
	}, AuditPage{PageSize: 100})

	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total != 2 {
		t.Errorf("Expected 2 login records, got %d", total)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records returned, got %d", len(records))
	}

	// Filter by actor
	records, total, err = store.List(AuditFilter{
		ActorID: "user-1",
	}, AuditPage{PageSize: 100})

	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total != 2 {
		t.Errorf("Expected 2 records for user-1, got %d", total)
	}
}

func TestAuditService_ValidateChain(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create a valid chain
	for i := 0; i < 5; i++ {
		_, err := service.Log(ctx, EventFunctionInvoke,
			WithActorID("user-1", "user", "Test"),
		)
		if err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	}

	// Validate chain
	result, err := service.ValidateChain(1, 5)
	if err != nil {
		t.Fatalf("ValidateChain failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("Chain should be valid, errors: %v", result.Errors)
	}

	if result.TotalRecords != 5 {
		t.Errorf("Expected 5 records, got %d", result.TotalRecords)
	}
}

func TestAuditService_Stats(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create various records
	service.Log(ctx, EventLogin, WithActorID("user-1", "user", "User 1"))
	service.Log(ctx, EventLogin, WithActorID("user-2", "user", "User 2"))
	service.Log(ctx, EventAccessDenied, WithActorID("user-1", "user", "User 1"), WithOutcome("failure", "access denied"))
	service.Log(ctx, EventFunctionInvoke, WithActorID("user-1", "user", "User 1"))

	// Get stats
	now := time.Now()
	start := now.Add(-24 * time.Hour)
	stats, err := service.GetStats(start, now)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalRecords != 4 {
		t.Errorf("Expected 4 total records, got %d", stats.TotalRecords)
	}

	if stats.ByEventType[EventLogin] != 2 {
		t.Errorf("Expected 2 login events, got %d", stats.ByEventType[EventLogin])
	}

	if stats.FailureRate <= 0 {
		t.Error("Expected non-zero failure rate")
	}
}

func TestAuditService_Export(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create records
	for i := 0; i < 3; i++ {
		_, err := service.Log(ctx, EventLogin,
			WithActorID("user-1", "user", "Test"),
		)
		if err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	}

	// Export as JSON
	data, err := service.Export(AuditFilter{}, "json")
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Export data should not be empty")
	}

	// Export as CSV
	data, err = service.Export(AuditFilter{}, "csv")
	if err != nil {
		t.Fatalf("Export CSV failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("CSV export data should not be empty")
	}
}

func TestAuditService_CategoryInference(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	tests := []struct {
		eventType     AuditEventType
		expectedCategory AuditCategory
	}{
		{EventLogin, CategorySecurity},
		{EventAccessDenied, CategorySecurity},
		{EventUserCreate, CategoryAdmin},
		{EventFunctionInvoke, CategoryOperational},
		{EventDataExport, CategoryData},
	}

	for _, tt := range tests {
		record, err := service.Log(ctx, tt.eventType,
			WithActorID("user-1", "user", "Test"),
		)
		if err != nil {
			t.Fatalf("Log failed for %s: %v", tt.eventType, err)
		}

		if record.Category != tt.expectedCategory {
			t.Errorf("Event %s: expected category %s, got %s",
				tt.eventType, tt.expectedCategory, record.Category)
		}
	}
}

func TestAuditService_SeverityInference(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	tests := []struct {
		eventType        AuditEventType
		expectedSeverity AuditSeverity
	}{
		{EventLogin, SeverityInfo},
		{EventLoginFailed, SeverityWarning},
		{EventAccessDenied, SeverityWarning},
		{EventUserDelete, SeverityCritical},
	}

	for _, tt := range tests {
		record, err := service.Log(ctx, tt.eventType,
			WithActorID("user-1", "user", "Test"),
		)
		if err != nil {
			t.Fatalf("Log failed for %s: %v", tt.eventType, err)
		}

		if record.Severity != tt.expectedSeverity {
			t.Errorf("Event %s: expected severity %s, got %s",
				tt.eventType, tt.expectedSeverity, record.Severity)
		}
	}
}

func TestAuditService_Pagination(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create 25 records
	for i := 0; i < 25; i++ {
		_, err := service.Log(ctx, EventFunctionInvoke,
			WithActorID("user-1", "user", "Test"),
		)
		if err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	}

	// Get first page
	records, total, err := store.List(AuditFilter{}, AuditPage{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total != 25 {
		t.Errorf("Expected total 25, got %d", total)
	}

	if len(records) != 10 {
		t.Errorf("Expected 10 records on page 1, got %d", len(records))
	}

	// Get last page
	records, _, err = store.List(AuditFilter{}, AuditPage{Page: 3, PageSize: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(records) != 5 {
		t.Errorf("Expected 5 records on page 3, got %d", len(records))
	}
}

func TestAuditService_TimeFilter(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create a record
	record1, _ := service.Log(ctx, EventLogin, WithActorID("user-1", "user", "Test"))

	// Wait and create another
	time.Sleep(10 * time.Millisecond)
	record2, _ := service.Log(ctx, EventLogin, WithActorID("user-2", "user", "Test"))

	// Filter by time
	start := record1.Timestamp.Add(5 * time.Millisecond)
	records, total, err := store.List(AuditFilter{
		StartTime: &start,
	}, AuditPage{PageSize: 100})

	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total != 1 {
		t.Errorf("Expected 1 record after time filter, got %d", total)
	}

	if len(records) > 0 && records[0].ID != record2.ID {
		t.Error("Expected to get record2")
	}
}

func TestAuditService_Archive(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := context.Background()

	// Create records
	for i := 0; i < 5; i++ {
		_, err := service.Log(ctx, EventFunctionInvoke,
			WithActorID("user-1", "user", "Test"),
		)
		if err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	}

	// Get initial count
	_, total, _ := store.List(AuditFilter{}, AuditPage{PageSize: 100})
	if total != 5 {
		t.Fatalf("Expected 5 records, got %d", total)
	}

	// Archive old records (archive everything before now + 1 hour)
	archiveTime := time.Now().Add(time.Hour)
	count, err := service.Archive(archiveTime, "/archive/audit.json")
	if err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 archived records, got %d", count)
	}

	// Verify records are deleted
	_, total, _ = store.List(AuditFilter{}, AuditPage{PageSize: 100})
	if total != 0 {
		t.Errorf("Expected 0 records after archive, got %d", total)
	}
}
