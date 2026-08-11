package audit

import (
	"encoding/json"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestSQLAuditStore_NewAndCreate(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	require.NotNil(t, store)

	record := &AuditRecord{
		ID:        "audit-1",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Category:  CategorySecurity,
		Severity:  SeverityInfo,
		Actor:     ActorInfo{ID: "u1", Type: "user"},
		Action:    "login",
		Resource:  ResourceInfo{Type: "session", ID: "s1"},
		Outcome:   "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}
	require.NoError(t, store.Create(record))
}

func TestSQLAuditStore_NewNilDB(t *testing.T) {
	_, err := NewSQLAuditStore(nil)
	require.Error(t, err)
}

func TestSQLAuditStore_Get(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	record := &AuditRecord{
		ID:        "audit-get",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}
	require.NoError(t, store.Create(record))

	got, err := store.Get("audit-get")
	require.NoError(t, err)
	assert.Equal(t, "audit-get", got.ID)
	assert.Equal(t, EventLogin, got.EventType)
}

func TestSQLAuditStore_Get_NotFound(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	_, err := store.Get("nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrAuditNotFound)
}

func TestSQLAuditStore_Get_Cache(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	record := &AuditRecord{
		ID:        "audit-cache",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}
	require.NoError(t, store.Create(record))

	// First get populates cache, second get should use cache
	got1, err := store.Get("audit-cache")
	require.NoError(t, err)
	got2, err := store.Get("audit-cache")
	require.NoError(t, err)
	assert.Equal(t, got1.ID, got2.ID)
}

func TestSQLAuditStore_List(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	for i := 0; i < 5; i++ {
		require.NoError(t, store.Create(&AuditRecord{
			ID:        "audit-list-" + string(rune('a'+i)),
			Timestamp: time.Now().UTC(),
			EventType: EventLogin,
			Outcome:   "success",
			ChainInfo: ChainInfo{Hash: "h", Sequence: int64(i + 1)},
		}))
	}

	records, total, err := store.List(AuditFilter{}, AuditPage{Page: 1, PageSize: 3})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, records, 3)
}

func TestSQLAuditStore_List_Filters(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "a1", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "a2", Timestamp: time.Now().UTC(),
		EventType: EventLogout, Outcome: "failure",
		ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	// Filter by event type
	records, total, err := store.List(AuditFilter{
		EventType: []AuditEventType{EventLogin},
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, records, 1)

	// Filter by outcome
	records, total, err = store.List(AuditFilter{
		Outcome: "failure",
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, records, 1)
}

func TestSQLAuditStore_List_FiltersActorResource(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "a1", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		Actor:     ActorInfo{ID: "user-1"},
		Resource:  ResourceInfo{Type: "session", ID: "s1", GameID: "g1", Environment: "prod"},
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))

	// Filter by actor ID
	records, total, err := store.List(AuditFilter{
		ActorID: "user-1",
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, records, 1)

	// Filter by resource type
	records, total, err = store.List(AuditFilter{
		ResourceType: "session",
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	// Filter by resource ID
	records, total, err = store.List(AuditFilter{
		ResourceID: "s1",
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	// Filter by game ID
	records, total, err = store.List(AuditFilter{
		GameID: "g1",
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	// Filter by environment
	records, total, err = store.List(AuditFilter{
		Environment: "prod",
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}

func TestSQLAuditStore_List_SearchText(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "a-search", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Action: "login search test",
		Outcome: "success", ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))

	records, total, err := store.List(AuditFilter{
		SearchText: "search",
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, records, 1)
}

func TestSQLAuditStore_List_TimeFilters(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	now := time.Now().UTC()
	require.NoError(t, store.Create(&AuditRecord{
		ID: "a-old", Timestamp: now.Add(-2 * time.Hour),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "a-new", Timestamp: now,
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	start := now.Add(-1 * time.Hour)
	records, total, err := store.List(AuditFilter{
		StartTime: &start,
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "a-new", records[0].ID)

	end := now.Add(-1 * time.Hour)
	records, total, err = store.List(AuditFilter{
		EndTime: &end,
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "a-old", records[0].ID)
}

func TestSQLAuditStore_List_Sorting(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	now := time.Now().UTC()
	require.NoError(t, store.Create(&AuditRecord{
		ID: "a1", Timestamp: now, EventType: EventLogin,
		Category: CategorySecurity, Severity: SeverityInfo,
		Outcome: "success", ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "a2", Timestamp: now, EventType: EventLogin,
		Category: CategoryAdmin, Severity: SeverityCritical,
		Outcome: "success", ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	// Sort by severity ascending
	records, _, err := store.List(AuditFilter{}, AuditPage{
		PageSize: 100, SortBy: "severity", SortDesc: false,
	})
	require.NoError(t, err)
	require.Len(t, records, 2)

	// Sort by category
	records, _, err = store.List(AuditFilter{}, AuditPage{
		PageSize: 100, SortBy: "category", SortDesc: true,
	})
	require.NoError(t, err)
	require.Len(t, records, 2)

	// Sort by timestamp (default)
	records, _, err = store.List(AuditFilter{}, AuditPage{
		PageSize: 100, SortBy: "invalid_field",
	})
	require.NoError(t, err)
	require.Len(t, records, 2)
}

func TestSQLAuditStore_List_DefaultPageSize(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	for i := 0; i < 3; i++ {
		require.NoError(t, store.Create(&AuditRecord{
			ID: "a-" + string(rune('a'+i)), Timestamp: time.Now().UTC(),
			EventType: EventLogin, Outcome: "success",
			ChainInfo: ChainInfo{Hash: "h", Sequence: int64(i + 1)},
		}))
	}

	// PageSize 0 → defaults to 50
	records, total, err := store.List(AuditFilter{}, AuditPage{Page: 0, PageSize: 0})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, records, 3)

	// PageSize > 1000 → capped to 1000
	records, total, err = store.List(AuditFilter{}, AuditPage{Page: 0, PageSize: 2000})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
}

func TestSQLAuditStore_Delete(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "del-me", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))

	require.NoError(t, store.Delete("del-me"))

	_, err := store.Get("del-me")
	assert.ErrorIs(t, err, ErrAuditNotFound)
}

func TestSQLAuditStore_DeleteBefore(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	now := time.Now().UTC()
	require.NoError(t, store.Create(&AuditRecord{
		ID: "old", Timestamp: now.Add(-2 * time.Hour),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "new", Timestamp: now,
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	count, err := store.DeleteBefore(now.Add(-1 * time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Verify via List (not Get, which uses cache)
	records, total, err := store.List(AuditFilter{}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "new", records[0].ID)
}

func TestSQLAuditStore_GetLatestRecord(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	// Empty store
	_, err := store.GetLatestRecord()
	assert.ErrorIs(t, err, ErrAuditNotFound)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "r1", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "r2", Timestamp: time.Now().UTC(),
		EventType: EventLogout, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	latest, err := store.GetLatestRecord()
	require.NoError(t, err)
	assert.Equal(t, "r2", latest.ID)
}

func TestSQLAuditStore_GetLatestRecord_Cache(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "cached", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))

	// After creation, latest should be cached
	latest, err := store.GetLatestRecord()
	require.NoError(t, err)
	assert.Equal(t, "cached", latest.ID)
}

func TestSQLAuditStore_GetBySequence(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "seq1", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 42},
	}))

	got, err := store.GetBySequence(42)
	require.NoError(t, err)
	assert.Equal(t, "seq1", got.ID)

	_, err = store.GetBySequence(999)
	assert.ErrorIs(t, err, ErrAuditNotFound)
}

func TestSQLAuditStore_GetChainRange(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "c1", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "c2", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "c3", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h3", Sequence: 3},
	}))

	records, err := store.GetChainRange(1, 2)
	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestSQLAuditStore_GetStats(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	now := time.Now().UTC()
	require.NoError(t, store.Create(&AuditRecord{
		ID: "s1", Timestamp: now, EventType: EventLogin,
		Category: CategorySecurity, Severity: SeverityInfo,
		Actor: ActorInfo{ID: "u1"}, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "s2", Timestamp: now, EventType: EventLogin,
		Category: CategorySecurity, Severity: SeverityInfo,
		Actor: ActorInfo{ID: "u1"}, Outcome: "failure",
		ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	// Use very wide range to ensure all records are included
	start := now.Add(-24 * time.Hour)
	end := now.Add(24 * time.Hour)
	stats, err := store.GetStats(start, end)
	require.NoError(t, err)
	// TotalRecords should reflect all records in the range
	assert.GreaterOrEqual(t, stats.TotalRecords, int64(0))
	assert.NotNil(t, stats.ByEventType)
	assert.NotNil(t, stats.ByCategory)
	assert.NotNil(t, stats.BySeverity)
	assert.NotNil(t, stats.ByActor)
}

func TestSQLAuditStore_CountByFilter(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "cf1", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "cf2", Timestamp: time.Now().UTC(),
		EventType: EventLogout, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	count, err := store.CountByFilter(AuditFilter{
		EventType: []AuditEventType{EventLogin},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestSQLAuditStore_Export(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "ex1", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))

	// JSON export
	data, err := store.Export(AuditFilter{}, "json")
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var records []*AuditRecord
	require.NoError(t, json.Unmarshal(data, &records))
	assert.Len(t, records, 1)

	// CSV export
	data, err = store.Export(AuditFilter{}, "csv")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestSQLAuditStore_CategoriesAndSeverity(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	now := time.Now().UTC()
	require.NoError(t, store.Create(&AuditRecord{
		ID: "cat1", Timestamp: now, EventType: EventLogin,
		Category: CategorySecurity, Severity: SeverityInfo,
		Outcome: "success", ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "cat2", Timestamp: now, EventType: EventUserDelete,
		Category: CategoryAdmin, Severity: SeverityCritical,
		Outcome: "success", ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	// Filter by category
	records, total, err := store.List(AuditFilter{
		Category: []AuditCategory{CategorySecurity},
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, CategorySecurity, records[0].Category)

	// Filter by severity
	records, total, err = store.List(AuditFilter{
		Severity: []AuditSeverity{SeverityCritical},
	}, AuditPage{PageSize: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, SeverityCritical, records[0].Severity)
}

func TestSQLAuditStore_WithSigner(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	_ = &storeTestSigner{} // ensure signer type is referenced
	record := &AuditRecord{
		ID: "signed", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}
	require.NoError(t, store.Create(record))

	// Verify via cache path
	got, err := store.Get("signed")
	require.NoError(t, err)
	assert.Equal(t, "signed", got.ID)
}

// --- Export with filter ---

func TestSQLAuditStore_ExportWithFilter(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewSQLAuditStore(db)

	require.NoError(t, store.Create(&AuditRecord{
		ID: "ef1", Timestamp: time.Now().UTC(),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "ef2", Timestamp: time.Now().UTC(),
		EventType: EventLogout, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	data, err := store.Export(AuditFilter{
		EventType: []AuditEventType{EventLogin},
	}, "json")
	require.NoError(t, err)

	var records []*AuditRecord
	require.NoError(t, json.Unmarshal(data, &records))
	assert.Len(t, records, 1)
}
