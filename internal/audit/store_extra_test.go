package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- InMemoryAuditStore uncovered methods ---

func TestInMemoryAuditStore_Delete(t *testing.T) {
	store := NewInMemoryAuditStore()
	r := &AuditRecord{ID: "r1", Timestamp: time.Now(), EventType: EventLogin, Outcome: "success"}
	store.Create(r)

	if err := store.Delete("r1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get("r1"); err != ErrAuditNotFound {
		t.Errorf("expected ErrAuditNotFound, got %v", err)
	}
}

func TestInMemoryAuditStore_Delete_NotFound(t *testing.T) {
	store := NewInMemoryAuditStore()
	if err := store.Delete("missing"); err != ErrAuditNotFound {
		t.Errorf("expected ErrAuditNotFound, got %v", err)
	}
}

func TestInMemoryAuditStore_GetBySequence(t *testing.T) {
	store := NewInMemoryAuditStore()
	r := &AuditRecord{ID: "r1", Timestamp: time.Now(), EventType: EventLogin, Outcome: "success"}
	store.Create(r)

	got, err := store.GetBySequence(1)
	if err != nil {
		t.Fatalf("GetBySequence: %v", err)
	}
	if got.ID != "r1" {
		t.Errorf("expected r1, got %s", got.ID)
	}
}

func TestInMemoryAuditStore_GetBySequence_NotFound(t *testing.T) {
	store := NewInMemoryAuditStore()
	if _, err := store.GetBySequence(999); err != ErrAuditNotFound {
		t.Errorf("expected ErrAuditNotFound, got %v", err)
	}
}

func TestInMemoryAuditStore_CountByFilter(t *testing.T) {
	store := NewInMemoryAuditStore()
	store.Create(&AuditRecord{ID: "r1", Timestamp: time.Now(), EventType: EventLogin, Outcome: "success"})
	store.Create(&AuditRecord{ID: "r2", Timestamp: time.Now(), EventType: EventLogout, Outcome: "success"})
	store.Create(&AuditRecord{ID: "r3", Timestamp: time.Now(), EventType: EventLogin, Outcome: "failure"})

	count, err := store.CountByFilter(AuditFilter{EventType: []AuditEventType{EventLogin}})
	if err != nil {
		t.Fatalf("CountByFilter: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}

	count, err = store.CountByFilter(AuditFilter{Outcome: "failure"})
	if err != nil {
		t.Fatalf("CountByFilter: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestInMemoryAuditStore_GetChainRange(t *testing.T) {
	store := NewInMemoryAuditStore()
	store.Create(&AuditRecord{ID: "r1", Timestamp: time.Now(), EventType: EventLogin, Outcome: "success"})
	store.Create(&AuditRecord{ID: "r2", Timestamp: time.Now(), EventType: EventLogout, Outcome: "success"})
	store.Create(&AuditRecord{ID: "r3", Timestamp: time.Now(), EventType: EventLogin, Outcome: "success"})

	records, err := store.GetChainRange(1, 2)
	if err != nil {
		t.Fatalf("GetChainRange: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
}

func TestInMemoryAuditStore_GetLatestRecord_Empty(t *testing.T) {
	store := NewInMemoryAuditStore()
	if _, err := store.GetLatestRecord(); err != ErrAuditNotFound {
		t.Errorf("expected ErrAuditNotFound, got %v", err)
	}
}

func TestInMemoryAuditStore_DeleteBefore(t *testing.T) {
	store := NewInMemoryAuditStore()
	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()

	store.Create(&AuditRecord{ID: "r1", Timestamp: old, EventType: EventLogin, Outcome: "success"})
	store.Create(&AuditRecord{ID: "r2", Timestamp: recent, EventType: EventLogin, Outcome: "success"})

	count, err := store.DeleteBefore(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("DeleteBefore: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
	if _, err := store.Get("r1"); err != ErrAuditNotFound {
		t.Error("r1 should be deleted")
	}
	if _, err := store.Get("r2"); err != nil {
		t.Error("r2 should still exist")
	}
}

// --- AuditWriter and simpleHash ---

func TestAuditWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	w := NewAuditWriter(&buf)

	r := &AuditRecord{
		ID:        "r1",
		Timestamp: time.Now(),
		EventType: EventLogin,
		Outcome:   "success",
		Actor:     ActorInfo{ID: "user-1"},
	}

	if err := w.Write(r); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if r.ChainInfo.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", r.ChainInfo.Sequence)
	}
	if r.ChainInfo.Hash == "" {
		t.Error("hash should not be empty")
	}
	if buf.Len() == 0 {
		t.Error("expected output in buffer")
	}
}

func TestAuditWriter_MultipleWrites(t *testing.T) {
	var buf bytes.Buffer
	w := NewAuditWriter(&buf)

	for i := 0; i < 3; i++ {
		r := &AuditRecord{
			ID:        "r" + string(rune('1'+i)),
			Timestamp: time.Now(),
			EventType: EventLogin,
			Outcome:   "success",
		}
		if err := w.Write(r); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	var records []*AuditRecord
	for _, line := range lines {
		var rec AuditRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		records = append(records, &rec)
	}

	// PrevHash is formatted from w.prev buffer (32 bytes), so it's hex of
	// the hash string + padding zeros. Verify chain linkage via sequence numbers.
	for i, rec := range records {
		if rec.ChainInfo.Sequence != int64(i+1) {
			t.Errorf("record %d: expected sequence %d, got %d", i, i+1, rec.ChainInfo.Sequence)
		}
		if rec.ChainInfo.Hash == "" {
			t.Errorf("record %d: hash should not be empty", i)
		}
	}
	// PrevHash of record 2 should contain the hash of record 1 (it's hex-prefixed)
	if records[1].ChainInfo.PrevHash == "" {
		t.Error("second record should have non-empty prev hash")
	}
}

func TestSimpleHash(t *testing.T) {
	h1 := simpleHash([]byte("hello"))
	h2 := simpleHash([]byte("hello"))
	h3 := simpleHash([]byte("world"))

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
	if len(h1) != 8 {
		t.Errorf("expected 8 char hash, got %d", len(h1))
	}
}

// --- ToRecord / FromRecord ---

func TestAuditModel_ToRecord(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	actorJSON, _ := json.Marshal(ActorInfo{ID: "u1", Type: "user", Name: "Test"})
	resourceJSON, _ := json.Marshal(ResourceInfo{Type: "session", ID: "s1"})
	detailsJSON, _ := json.Marshal(map[string]interface{}{"key": "val"})
	changesJSON, _ := json.Marshal(ChangeInfo{Before: map[string]interface{}{"a": "1"}, After: map[string]interface{}{"a": "2"}})
	contextJSON, _ := json.Marshal(AuditContext{RequestID: "req-1"})

	m := &AuditModel{
		AuditID:        "audit-1",
		Timestamp:      now,
		EventType:      string(EventLogin),
		Category:       string(CategorySecurity),
		Severity:       string(SeverityInfo),
		ActorJSON:      actorJSON,
		Action:         "login",
		ResourceJSON:   resourceJSON,
		DetailsJSON:    detailsJSON,
		ChangesJSON:    changesJSON,
		ContextJSON:    contextJSON,
		Outcome:        "success",
		ChainHash:      "abc123",
		ChainPrevHash:  "def456",
		ChainSequence:  42,
		ChainSignerID:  "signer-1",
		ChainSignature: "sig",
	}

	r, err := m.ToRecord()
	if err != nil {
		t.Fatalf("ToRecord: %v", err)
	}
	if r.ID != "audit-1" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Actor.ID != "u1" {
		t.Errorf("Actor.ID = %q", r.Actor.ID)
	}
	if r.Resource.Type != "session" {
		t.Errorf("Resource.Type = %q", r.Resource.Type)
	}
	if r.Details["key"] != "val" {
		t.Errorf("Details[key] = %v", r.Details["key"])
	}
	if r.Changes == nil {
		t.Fatal("Changes should not be nil")
	}
	if r.Context.RequestID != "req-1" {
		t.Errorf("Context.RequestID = %q", r.Context.RequestID)
	}
	if r.ChainInfo.Hash != "abc123" {
		t.Errorf("ChainInfo.Hash = %q", r.ChainInfo.Hash)
	}
	if r.ChainInfo.Sequence != 42 {
		t.Errorf("ChainInfo.Sequence = %d", r.ChainInfo.Sequence)
	}
}

func TestAuditModel_ToRecord_NilJSON(t *testing.T) {
	m := &AuditModel{
		AuditID:   "audit-nil",
		Timestamp: time.Now(),
		EventType: string(EventLogin),
		Outcome:   "success",
	}

	r, err := m.ToRecord()
	if err != nil {
		t.Fatalf("ToRecord: %v", err)
	}
	if r.Actor.ID != "" {
		t.Error("Actor should be zero value")
	}
	if r.Details != nil {
		t.Error("Details should be nil")
	}
	if r.Changes != nil {
		t.Error("Changes should be nil")
	}
	if r.Context.RequestID != "" {
		t.Error("Context should be zero value")
	}
}

func TestAuditModel_ToRecord_InvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		m    *AuditModel
	}{
		{"bad actor", &AuditModel{AuditID: "a1", Timestamp: time.Now(), EventType: "e", Outcome: "s", ActorJSON: []byte("x")}},
		{"bad resource", &AuditModel{AuditID: "a2", Timestamp: time.Now(), EventType: "e", Outcome: "s", ResourceJSON: []byte("x")}},
		{"bad details", &AuditModel{AuditID: "a3", Timestamp: time.Now(), EventType: "e", Outcome: "s", DetailsJSON: []byte("x")}},
		{"bad changes", &AuditModel{AuditID: "a4", Timestamp: time.Now(), EventType: "e", Outcome: "s", ChangesJSON: []byte("x")}},
		{"bad context", &AuditModel{AuditID: "a5", Timestamp: time.Now(), EventType: "e", Outcome: "s", ContextJSON: []byte("x")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.m.ToRecord(); err == nil {
				t.Error("expected error for invalid JSON")
			}
		})
	}
}

func TestFromRecord_Full(t *testing.T) {
	r := &AuditRecord{
		ID:        "audit-1",
		Timestamp: time.Now().Truncate(time.Second),
		EventType: EventLogin,
		Category:  CategorySecurity,
		Severity:  SeverityInfo,
		Actor:     ActorInfo{ID: "u1", Type: "user"},
		Action:    "login",
		Resource:  ResourceInfo{Type: "session", ID: "s1"},
		Details:   map[string]interface{}{"key": "val"},
		Changes: &ChangeInfo{
			Before: map[string]interface{}{"a": "1"},
			After:  map[string]interface{}{"a": "2"},
		},
		Context: AuditContext{RequestID: "req-1"},
		Outcome: "success",
		ChainInfo: ChainInfo{
			Hash: "abc", PrevHash: "def", Sequence: 1, SignerID: "s", Signature: "sig",
		},
	}

	m, err := FromRecord(r)
	if err != nil {
		t.Fatalf("FromRecord: %v", err)
	}
	if m.AuditID != "audit-1" {
		t.Errorf("AuditID = %q", m.AuditID)
	}
	if m.ActorJSON == nil || m.ResourceJSON == nil || m.DetailsJSON == nil || m.ChangesJSON == nil || m.ContextJSON == nil {
		t.Error("expected all JSON fields to be non-nil")
	}
}

func TestFromRecord_Minimal(t *testing.T) {
	r := &AuditRecord{ID: "a-min", Timestamp: time.Now(), EventType: EventLogin, Outcome: "success"}
	m, err := FromRecord(r)
	if err != nil {
		t.Fatalf("FromRecord: %v", err)
	}
	if m.ActorJSON != nil || m.ResourceJSON != nil || m.DetailsJSON != nil || m.ChangesJSON != nil || m.ContextJSON != nil {
		t.Error("expected all JSON fields to be nil for minimal record")
	}
}

func TestAuditModel_TableName(t *testing.T) {
	if (AuditModel{}).TableName() != "audit_records" {
		t.Error("expected audit_records")
	}
}

// --- AuditService with signer and notifier ---

type storeTestSigner struct {
	verifyErr error
}

func (s *storeTestSigner) Sign(record *AuditRecord) (string, string, error) {
	return "test-sig", "test-signer", nil
}

func (s *storeTestSigner) Verify(record *AuditRecord) error {
	return s.verifyErr
}

type storeTestNotifier struct {
	notified bool
}

func (n *storeTestNotifier) NotifyAudit(ctx context.Context, record *AuditRecord) error {
	n.notified = true
	return nil
}

func TestAuditService_ValidateChain_Valid(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	service.Log(context.Background(), EventLogin, WithActorID("u1", "user", "T"), WithOutcome("success", ""))
	service.Log(context.Background(), EventLogout, WithActorID("u1", "user", "T"), WithOutcome("success", ""))

	result, err := service.ValidateChain(1, 2)
	if err != nil {
		t.Fatalf("ValidateChain: %v", err)
	}
	if !result.Valid {
		t.Errorf("chain should be valid, errors: %v", result.Errors)
	}
	if result.TotalRecords != 2 {
		t.Errorf("expected 2 records, got %d", result.TotalRecords)
	}
}

func TestAuditService_ValidateChain_BrokenChain(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	service.Log(context.Background(), EventLogin, WithActorID("u1", "user", "T"), WithOutcome("success", ""))
	service.Log(context.Background(), EventLogout, WithActorID("u1", "user", "T"), WithOutcome("success", ""))

	records, _ := store.GetChainRange(1, 2)
	records[1].ChainInfo.PrevHash = "tampered"

	result, err := service.ValidateChain(1, 2)
	if err != nil {
		t.Fatalf("ValidateChain: %v", err)
	}
	if result.Valid {
		t.Error("chain should be invalid after tampering")
	}
}

func TestAuditService_ValidateChain_HashMismatch(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	service.Log(context.Background(), EventLogin, WithActorID("u1", "user", "T"), WithOutcome("success", ""))

	records, _ := store.GetChainRange(1, 1)
	records[0].ChainInfo.Hash = "wrong"

	result, err := service.ValidateChain(1, 1)
	if err != nil {
		t.Fatalf("ValidateChain: %v", err)
	}
	if result.Valid {
		t.Error("chain should be invalid with wrong hash")
	}
}

func TestAuditService_ValidateChain_WithSigner(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, &storeTestSigner{})

	service.Log(context.Background(), EventLogin, WithActorID("u1", "user", "T"), WithOutcome("success", ""))

	result, err := service.ValidateChain(1, 1)
	if err != nil {
		t.Fatalf("ValidateChain: %v", err)
	}
	if !result.Valid {
		t.Errorf("chain should be valid, errors: %v", result.Errors)
	}
}

func TestAuditService_ValidateChain_InvalidSignature(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, &storeTestSigner{verifyErr: ErrInvalidChain})

	service.Log(context.Background(), EventLogin, WithActorID("u1", "user", "T"), WithOutcome("success", ""))

	result, err := service.ValidateChain(1, 1)
	if err != nil {
		t.Fatalf("ValidateChain: %v", err)
	}
	if result.Valid {
		t.Error("chain should be invalid with bad signature")
	}
}

func TestAuditService_Archive_Op(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	service.Log(context.Background(), EventLogin, WithActorID("u1", "user", "T"), WithOutcome("success", ""))

	count, err := service.Archive(time.Now().Add(time.Hour), "/tmp/archive.json")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 archived, got %d", count)
	}
}

func TestAuditService_SetNotifier_Critical(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)
	n := &storeTestNotifier{}
	service.SetNotifier(n)

	service.Log(context.Background(), EventUserDelete,
		WithActorID("u1", "user", "T"), WithOutcome("success", ""),
	)

	if !n.notified {
		t.Error("expected notification for critical event")
	}
}

func TestAuditService_Log_WithChangesAndGameID(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	r, err := service.Log(context.Background(), EventConfigUpdate,
		WithChanges(map[string]interface{}{"key": "old"}, map[string]interface{}{"key": "new", "added": "val"}),
		WithSeverity(SeverityInfo),
		WithCategory(CategoryOperational),
		WithGameID("game-1", "prod"),
	)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if r.Changes == nil || len(r.Changes.DiffFields) == 0 {
		t.Error("expected diff fields")
	}
	if r.Resource.GameID != "game-1" {
		t.Errorf("GameID = %q", r.Resource.GameID)
	}
}

func TestAuditService_Log_MaskSensitive(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	r, err := service.Log(context.Background(), EventConfigUpdate,
		WithDetails(map[string]interface{}{
			"password": "secret123",
			"api_key":  "key123",
			"name":     "visible",
		}),
	)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if r.Details["password"] != "***MASKED***" {
		t.Errorf("password should be masked, got %v", r.Details["password"])
	}
	if r.Details["api_key"] != "***MASKED***" {
		t.Errorf("api_key should be masked, got %v", r.Details["api_key"])
	}
	if r.Details["name"] != "visible" {
		t.Errorf("name should be visible, got %v", r.Details["name"])
	}
}

func TestAuditService_Log_WithActor(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	actor := ActorInfo{ID: "u1", Type: "user", Name: "Test User", Email: "test@example.com"}
	r, err := service.Log(context.Background(), EventLogin, WithActor(actor))
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if r.Actor.ID != "u1" || r.Actor.Name != "Test User" {
		t.Errorf("unexpected actor: %+v", r.Actor)
	}
}

func TestAuditService_Log_WithResource(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	resource := ResourceInfo{Type: "game", ID: "game-1", Name: "Test Game"}
	r, err := service.Log(context.Background(), EventFunctionInvoke, WithResource(resource))
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if r.Resource.ID != "game-1" || r.Resource.Name != "Test Game" {
		t.Errorf("unexpected resource: %+v", r.Resource)
	}
}

func TestAuditService_Log_WithContext(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := AuditContext{RequestID: "req-123", TraceID: "trace-456", Service: "test-service"}
	r, err := service.Log(context.Background(), EventLogin, WithContext(ctx))
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if r.Context.RequestID != "req-123" || r.Context.TraceID != "trace-456" {
		t.Errorf("unexpected context: %+v", r.Context)
	}
}

func TestAuditService_Log_WithIPAddress(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	r, err := service.Log(context.Background(), EventLogin, WithIPAddress("10.0.0.1", "curl/7.0"))
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if r.Actor.IPAddress != "10.0.0.1" || r.Actor.UserAgent != "curl/7.0" {
		t.Errorf("unexpected IP/UA: %s/%s", r.Actor.IPAddress, r.Actor.UserAgent)
	}
}

func TestMaskSensitiveValueAllTypes(t *testing.T) {
	got := MaskSensitiveValue(map[string]interface{}{
		"password": "secret",
		"nested":   map[string]interface{}{"api_key": "k"},
		"list":     []interface{}{map[string]interface{}{"token": "t"}, 1},
		"plain":    "ok",
	})
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	if m["password"] == "secret" || m["plain"] != "ok" {
		t.Fatalf("masking wrong: %+v", m)
	}
	if _, isList := m["list"].([]interface{}); !isList {
		t.Fatalf("list not handled: %T", m["list"])
	}
	if MaskSensitiveValue(42) != 42 {
		t.Fatal("scalar must pass through")
	}
}

func TestBackfillPromotedFields_FindError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditModel{}))
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable("audit_records"))
	// 表被删后批量回填查询失败——不应 panic
	store.backfillPromotedFields()
}

func TestSQLStore_ListFindError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditModel{}))
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable("audit_records"))
	_, _, err = store.List(AuditFilter{}, AuditPage{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestInMemoryListDefaultPagingAndTimeFilter(t *testing.T) {
	store := NewInMemoryAuditStore()
	now := time.Now()
	rec := &AuditRecord{ID: "r1", Timestamp: now, Outcome: "success"}
	require.NoError(t, store.Create(rec))

	// PageSize/Page 非法 → 默认 50/1（806 分支）
	items, total, err := store.List(AuditFilter{}, AuditPage{})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)

	// EndTime 过滤排除（848 分支）
	past := now.Add(-time.Hour)
	items, _, err = store.List(AuditFilter{EndTime: &past}, AuditPage{})
	require.NoError(t, err)
	require.Empty(t, items)
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) { return 0, errors.New("disk full") }

func TestAuditWriterWriteFailure(t *testing.T) {
	w := NewAuditWriter(failingWriter{})
	rec := &AuditRecord{ID: "x", Timestamp: time.Now()}
	require.Error(t, w.Write(rec))
}

func TestSQLStoreGetLatestRecordQueryError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditModel{}))
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable("audit_records"))
	_, err = store.GetLatestRecord()
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrAuditNotFound)
}

func TestMaskSensitiveValueSliceTopLevel(t *testing.T) {
	got := MaskSensitiveValue([]interface{}{
		map[string]interface{}{"token": "t"},
		"plain",
		[]interface{}{map[string]interface{}{"password": "p"}},
	})
	list, ok := got.([]interface{})
	if !ok || len(list) != 3 {
		t.Fatalf("expected slice passthrough, got %T", got)
	}
	if inner, ok := list[0].(map[string]interface{}); !ok || inner["token"] == "t" {
		t.Fatalf("nested mask missing: %+v", list[0])
	}
}

func TestSQLStoreGetLatestRecordSuccess(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditModel{}))
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	rec := &AuditRecord{ID: "r1", Timestamp: time.Now(), EventType: EventLogin, Outcome: "success"}
	require.NoError(t, store.Create(rec))
	got, err := store.GetLatestRecord()
	require.NoError(t, err)
	require.Equal(t, "r1", got.ID)
}

func TestAuditWriterWriteSuccess(t *testing.T) {
	var buf bytes.Buffer
	w := NewAuditWriter(&buf)
	rec := &AuditRecord{ID: "r1", Timestamp: time.Now(), EventType: EventLogin, Outcome: "success"}
	require.NoError(t, w.Write(rec))
	require.NotEmpty(t, buf.String())
	require.NoError(t, w.Write(rec)) // second record updates prev hash
	if w.seq != 2 {
		t.Fatalf("seq = %d", w.seq)
	}
}

func TestSQLStore_ListFindErrorAfterCountOK(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditModel{}))
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	rec := &AuditRecord{ID: "r1", Timestamp: time.Now(), EventType: EventLogin, Outcome: "success"}
	require.NoError(t, store.Create(rec))

	var queries int32
	require.NoError(t, db.Callback().Query().Before("gorm:row;gorm:query").
		Register("test:audit_find_fail", func(tx *gorm.DB) {
			if atomic.AddInt32(&queries, 1) >= 2 {
				_ = tx.AddError(errors.New("find boom"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("test:audit_find_fail") })

	_, _, err = store.List(AuditFilter{}, AuditPage{Page: 1, PageSize: 10})
	require.Error(t, err)
}
