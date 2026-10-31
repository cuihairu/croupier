package audit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleRecord builds a fully-populated version-2 style record. Timestamp is
// millisecond-truncated so the DB round-trip is lossless.
func sampleRecord() *AuditRecord {
	return &AuditRecord{
		ID:        "audit-chain-v2-1",
		Timestamp: time.Date(2026, 10, 29, 12, 0, 0, 123_000_000, time.UTC),
		EventType: EventLogin,
		Category:  CategorySecurity,
		Severity:  SeverityWarning,
		Actor: ActorInfo{
			ID:        "u-1",
			Type:      "user",
			Name:      "alice",
			Email:     "alice@example.com",
			Roles:     []string{"admin", "operator"},
			IPAddress: "10.0.0.1",
			MFAUsed:   true,
			Metadata:  map[string]string{"k": "v"},
		},
		Action:   "login",
		Resource: ResourceInfo{Type: "session", ID: "s-1", GameID: "demo", Environment: "prod", Metadata: map[string]string{"m": "n"}},
		Outcome:  "success",
		Details:  map[string]interface{}{"attempt": 3, "note": "n"},
		Changes: &ChangeInfo{
			Before:     map[string]interface{}{"x": 1},
			After:      map[string]interface{}{"x": 2},
			DiffFields: []string{"x"},
		},
		Context: AuditContext{RequestID: "req-1", Service: "server", Environment: "prod", Tags: map[string]string{"t": "u"}},
		ChainInfo: ChainInfo{
			PrevHash:     "prev",
			Sequence:     5,
			ChainVersion: ChainVersionCanonical,
		},
	}
}

func TestCalculateHash_CanonicalTamperDetection(t *testing.T) {
	svc := NewAuditService(NewInMemoryAuditStore(), nil)

	base, err := svc.calculateHash(sampleRecord())
	require.NoError(t, err)

	cases := []struct {
		name   string
		mutate func(*AuditRecord)
	}{
		{"category", func(r *AuditRecord) { r.Category = CategoryCompliance }},
		{"severity", func(r *AuditRecord) { r.Severity = SeverityCritical }},
		{"action", func(r *AuditRecord) { r.Action = "logout" }},
		{"outcome", func(r *AuditRecord) { r.Outcome = "failure" }},
		{"errorMessage", func(r *AuditRecord) { r.ErrorMessage = "tampered" }},
		{"resource.gameId", func(r *AuditRecord) { r.Resource.GameID = "other-game" }},
		{"resource.environment", func(r *AuditRecord) { r.Resource.Environment = "staging" }},
		{"resource.metadata", func(r *AuditRecord) { r.Resource.Metadata = map[string]string{"m": "tampered"} }},
		{"actor.name", func(r *AuditRecord) { r.Actor.Name = "eve" }},
		{"actor.metadata", func(r *AuditRecord) { r.Actor.Metadata = map[string]string{"k": "tampered"} }},
		{"actor.roles", func(r *AuditRecord) { r.Actor.Roles = []string{"hacker"} }},
		{"changes", func(r *AuditRecord) { r.Changes.Before = map[string]interface{}{"x": 99} }},
		{"context", func(r *AuditRecord) { r.Context.Service = "tampered" }},
		{"details", func(r *AuditRecord) { r.Details["attempt"] = 4 }},
		{"id", func(r *AuditRecord) { r.ID = "audit-other" }},
		{"timestamp", func(r *AuditRecord) { r.Timestamp = r.Timestamp.Add(time.Second) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := sampleRecord()
			tc.mutate(r)
			got, err := svc.calculateHash(r)
			require.NoError(t, err)
			assert.NotEqual(t, base, got, "tampering %s must change the canonical hash", tc.name)
		})
	}
}

func TestCalculateHash_LegacyFieldSetPreserved(t *testing.T) {
	svc := NewAuditService(NewInMemoryAuditStore(), nil)

	// Version 0 / 1 uses the legacy 7-field subset: action/category/severity
	// are NOT hashed (the documented ARCH-007 gap), while outcome is.
	legacy := sampleRecord()
	legacy.ChainInfo.ChainVersion = ChainVersionLegacy
	legacyHash, err := svc.calculateHash(legacy)
	require.NoError(t, err)

	mutated := *legacy
	mutated.Action = "logout"
	unchanged, err := svc.calculateHash(&mutated)
	require.NoError(t, err)
	assert.Equal(t, legacyHash, unchanged, "legacy v1 digest must not cover action")

	mutated2 := *legacy
	mutated2.Outcome = "failure"
	changed, err := svc.calculateHash(&mutated2)
	require.NoError(t, err)
	assert.NotEqual(t, legacyHash, changed, "legacy v1 digest must still cover outcome")
}

func TestLog_WritesCanonicalVersion(t *testing.T) {
	svc := NewAuditService(NewInMemoryAuditStore(), nil)
	rec, err := svc.Log(context.Background(), EventLogin, WithActor(ActorInfo{ID: "u1", Type: "user"}), WithResource(ResourceInfo{Type: "session", ID: "s1"}))
	require.NoError(t, err)

	assert.Equal(t, ChainVersionCanonical, rec.ChainInfo.ChainVersion)
	assert.Equal(t, 64, len(rec.ChainInfo.Hash), "v2 hash must be 64-hex SHA-256")
	assert.NotContains(t, rec.ChainInfo.Hash, "-") // lowercase hex
}

func TestValidateChain_SQLRoundTripStable(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	svc := NewAuditService(store, nil)

	// Log three records with rich free-form payloads (numbers, maps, nested
	// maps, large int64 that becomes float64 after the JSON round-trip).
	for i := 0; i < 3; i++ {
		_, err := svc.Log(context.Background(), EventAccessGranted,
			WithActor(ActorInfo{ID: "u1", Type: "user", Name: "alice", Roles: []string{"admin"}, Metadata: map[string]string{"k": "v"}}),
			WithResource(ResourceInfo{Type: "role", ID: "r1", GameID: "demo", Environment: "prod"}),
			WithChanges(map[string]interface{}{"count": int64(1) << 40, "nested": map[string]interface{}{"a": float64(1.5)}},
				map[string]interface{}{"count": (int64(1) << 40) + 1, "nested": map[string]interface{}{"a": float64(2.5)}}),
			WithContext(AuditContext{RequestID: "req-1", Service: "server", Tags: map[string]string{"t": "u"}}),
		)
		require.NoError(t, err)
	}

	result, err := svc.ValidateChain(1, 3)
	require.NoError(t, err)
	assert.True(t, result.Valid, "round-trip must produce no false mismatch: %+v", result.Errors)
	assert.Equal(t, 3, result.TotalRecords)
}

func TestValidateChain_DetectsTamperedStoredRow(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	svc := NewAuditService(store, nil)

	rec, err := svc.Log(context.Background(), EventAccessGranted,
		WithActor(ActorInfo{ID: "u1", Type: "user"}),
		WithResource(ResourceInfo{Type: "role", ID: "r1"}))
	require.NoError(t, err)

	// Tamper the persisted row directly (e.g. an attacker edits the DB).
	var m AuditModel
	require.NoError(t, db.Where("audit_id = ?", rec.ID).First(&m).Error)
	m.Action = "tampered"
	require.NoError(t, db.Model(&m).Update("action", "tampered").Error)

	result, err := svc.ValidateChain(rec.ChainInfo.Sequence, rec.ChainInfo.Sequence)
	require.NoError(t, err)
	assert.False(t, result.Valid, "a tampered stored field must break the chain")
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "hash_mismatch", result.Errors[0].Type)
}

func TestValidateChain_LegacyV1RowStillValidates(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	svc := NewAuditService(store, nil)

	// Build a version-1 row the way pre-ARCH-008 code did: version 0 (unset),
	// ms-truncated timestamp, legacy hash. FromRecord maps version 0 -> 1.
	rec := sampleRecord()
	rec.ChainInfo = ChainInfo{}
	rec.ChainInfo.ChainVersion = 0
	require.NoError(t, svc.buildChainInfo(rec))
	require.NoError(t, store.Create(rec))

	// Read back through the SQL store (real DB round-trip) and validate.
	read, err := store.GetBySequence(rec.ChainInfo.Sequence)
	require.NoError(t, err)
	assert.Equal(t, ChainVersionLegacy, read.ChainInfo.ChainVersion)

	result, err := svc.ValidateChain(rec.ChainInfo.Sequence, rec.ChainInfo.Sequence)
	require.NoError(t, err)
	assert.True(t, result.Valid, "legacy v1 row must remain verifiable: %+v", result.Errors)
}

// failingStore wraps an in-memory store whose Create always fails.
type failingStore struct {
	*InMemoryAuditStore
	err error
}

func (f *failingStore) Create(_ *AuditRecord) error {
	return f.err
}

type countingNotifier struct {
	count int
	err   error
}

func (c *countingNotifier) NotifyAudit(_ context.Context, _ *AuditRecord) error {
	c.count++
	return c.err
}

func TestLog_WriteFailure_StrictPolicyDefault(t *testing.T) {
	store := &failingStore{InMemoryAuditStore: NewInMemoryAuditStore(), err: errors.New("audit db down")}
	svc := NewAuditService(store, nil)

	_, err := svc.Log(context.Background(), EventLogin, WithActor(ActorInfo{ID: "u1", Type: "user"}))
	require.Error(t, err, "strict (default) must fail the Log call")
	assert.Equal(t, WritePolicyStrict, svc.WriteFailurePolicy())
}

func TestLog_WriteFailure_BestEffortReturnsRecord(t *testing.T) {
	store := &failingStore{InMemoryAuditStore: NewInMemoryAuditStore(), err: errors.New("audit db down")}
	svc := NewAuditService(store, nil)
	svc.SetWriteFailurePolicy(string(WritePolicyBestEffort))
	notifier := &countingNotifier{}
	svc.SetNotifier(notifier)

	rec, err := svc.Log(context.Background(), EventLogin, WithActor(ActorInfo{ID: "u1", Type: "user"}))
	require.NoError(t, err, "best_effort must not fail the Log call")
	require.NotNil(t, rec)
	assert.True(t, rec.ChainInfo.WriteFailed)
	assert.Equal(t, 1, notifier.count, "first gap must alert")

	// Second failure within the rate-limit window must not alert again.
	_, err = svc.Log(context.Background(), EventLogin, WithActor(ActorInfo{ID: "u1", Type: "user"}))
	require.NoError(t, err)
	assert.Equal(t, 1, notifier.count, "rate-limited alert must fire once per window")
}

func TestSetWriteFailurePolicy_UnknownFallsBackToStrict(t *testing.T) {
	svc := NewAuditService(NewInMemoryAuditStore(), nil)
	svc.SetWriteFailurePolicy("bogus")
	assert.Equal(t, WritePolicyStrict, svc.WriteFailurePolicy())
	svc.SetWriteFailurePolicy("")
	assert.Equal(t, WritePolicyStrict, svc.WriteFailurePolicy())
}

func TestReportWriteGap_SurfacesGapAndRateLimits(t *testing.T) {
	svc := NewAuditService(NewInMemoryAuditStore(), nil)
	notifier := &countingNotifier{}
	svc.SetNotifier(notifier)

	// 副作用后审计写失败：呈现缺口（rate-limited 告警），不 panic、不返回 5xx。
	svc.ReportWriteGap(context.Background(), EventFunctionInvoke, errors.New("audit db down"))
	assert.Equal(t, 1, notifier.count, "first gap must alert")

	// 窗口内再次失败：rate-limited，不再告警。
	svc.ReportWriteGap(context.Background(), EventFunctionInvoke, errors.New("audit db down"))
	assert.Equal(t, 1, notifier.count, "rate-limited alert must fire once per window")
}

func TestCanonicalHashInput_Deterministic(t *testing.T) {
	a := sampleRecord()
	b := sampleRecord()

	ha, err := canonicalHashInput(a)
	require.NoError(t, err)
	hb, err := canonicalHashInput(b)
	require.NoError(t, err)
	assert.Equal(t, string(ha), string(hb), "identical records must produce identical canonical input")
}

func TestNormalizeHashPayload_RoundTripsNumbers(t *testing.T) {
	details := map[string]interface{}{
		"small": 100,            // int -> float64, marshals identically
		"big":   int64(1) << 40, // > 2^53, rounded on float64 conversion
		"pi":    3.14,
	}
	normalized := normalizeHashPayloadMap(details)

	// The original map must not be mutated (the caller-facing record keeps
	// its Go value shapes).
	if _, ok := details["big"].(int64); !ok {
		t.Fatal("normalizeHashPayloadMap must not mutate its input")
	}
	_, ok := normalized["big"].(float64)
	assert.True(t, ok, "numbers must be normalized to the DB float64 form")
	_, ok = normalized["small"].(float64)
	assert.True(t, ok, "small ints must be normalized too")

	// The normalized form must marshal byte-identically to a JSON round-trip
	// of itself (what ValidateChain sees after a DB read), and a DB-read copy
	// must produce the same canonical bytes as the original int64 map.
	canon, err := json.Marshal(map[string]interface{}{"details": normalized})
	require.NoError(t, err)
	data, err := json.Marshal(details)
	require.NoError(t, err)
	var readBack map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &readBack))
	canonFromRead, err := json.Marshal(map[string]interface{}{"details": normalizeHashPayloadMap(readBack)})
	require.NoError(t, err)
	assert.Equal(t, canon, canonFromRead, "int64 original and float64 read-back must canonicalize identically")
}
