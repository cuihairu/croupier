package audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/middleware/reqinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type v9FailingStore struct {
	inner            *InMemoryAuditStore
	getLatestErr     error
	createErr        error
	getChainRangeErr error
	exportErr        error
	deleteBeforeErr  error
}

func (s *v9FailingStore) Create(record *AuditRecord) error {
	if s.createErr != nil {
		return s.createErr
	}
	return s.inner.Create(record)
}

func (s *v9FailingStore) Get(id string) (*AuditRecord, error) {
	return s.inner.Get(id)
}

func (s *v9FailingStore) List(filter AuditFilter, page AuditPage) ([]*AuditRecord, int, error) {
	return s.inner.List(filter, page)
}

func (s *v9FailingStore) Delete(id string) error {
	return s.inner.Delete(id)
}

func (s *v9FailingStore) DeleteBefore(timestamp time.Time) (int64, error) {
	if s.deleteBeforeErr != nil {
		return 0, s.deleteBeforeErr
	}
	return s.inner.DeleteBefore(timestamp)
}

func (s *v9FailingStore) GetLatestRecord() (*AuditRecord, error) {
	if s.getLatestErr != nil {
		return nil, s.getLatestErr
	}
	return s.inner.GetLatestRecord()
}

func (s *v9FailingStore) GetBySequence(seq int64) (*AuditRecord, error) {
	return s.inner.GetBySequence(seq)
}

func (s *v9FailingStore) GetChainRange(startSeq, endSeq int64) ([]*AuditRecord, error) {
	if s.getChainRangeErr != nil {
		return nil, s.getChainRangeErr
	}
	return s.inner.GetChainRange(startSeq, endSeq)
}

func (s *v9FailingStore) GetStats(startTime, endTime time.Time) (*AuditStats, error) {
	return s.inner.GetStats(startTime, endTime)
}

func (s *v9FailingStore) CountByFilter(filter AuditFilter) (int64, error) {
	return s.inner.CountByFilter(filter)
}

func (s *v9FailingStore) Export(filter AuditFilter, format string) ([]byte, error) {
	if s.exportErr != nil {
		return nil, s.exportErr
	}
	return s.inner.Export(filter, format)
}

type v9FailingSigner struct {
	signErr   error
	verifyErr error
}

func (s *v9FailingSigner) Sign(record *AuditRecord) (string, string, error) {
	if s.signErr != nil {
		return "", "", s.signErr
	}
	return "sig", "signer", nil
}

func (s *v9FailingSigner) Verify(record *AuditRecord) error {
	return s.verifyErr
}

type v9ErrorWriter struct{}

func (v9ErrorWriter) Write(p []byte) (int, error) {
	return 0, errors.New("v9 write failed")
}

func TestAuditServiceStoreV9(t *testing.T) {
	var nilService *AuditService
	assert.Nil(t, nilService.Store())

	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)
	assert.Equal(t, store, service.Store())
}

func TestAuditServiceLogRequestInfoV9(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	ctx := reqinfo.WithContext(context.Background(), reqinfo.Info{
		IP:        "10.1.2.3",
		UserAgent: "v9-agent",
	})
	record, err := service.Log(ctx, EventLogin)
	require.NoError(t, err)
	assert.Equal(t, "10.1.2.3", record.Actor.IPAddress)
	assert.Equal(t, "v9-agent", record.Actor.UserAgent)

	nilCtxRecord, err := service.Log(nil, EventLogin)
	require.NoError(t, err)
	assert.Empty(t, nilCtxRecord.Actor.IPAddress)
}

func TestAuditServiceLogErrorPathsV9(t *testing.T) {
	createFail := &v9FailingStore{inner: NewInMemoryAuditStore(), createErr: errors.New("v9 create boom")}
	_, err := NewAuditService(createFail, nil).Log(context.Background(), EventLogin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store audit record")

	latestFail := &v9FailingStore{inner: NewInMemoryAuditStore(), getLatestErr: errors.New("v9 latest boom")}
	_, err = NewAuditService(latestFail, nil).Log(context.Background(), EventLogin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to build chain info")

	signFail := NewAuditService(NewInMemoryAuditStore(), &v9FailingSigner{signErr: errors.New("v9 sign boom")})
	_, err = signFail.Log(context.Background(), EventLogin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to build chain info")

	_, err = NewAuditService(NewInMemoryAuditStore(), nil).Log(context.Background(), EventLogin,
		WithDetails(map[string]interface{}{"ch": make(chan int)}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to build chain info")
}

func TestInferCategoryDefaultV9(t *testing.T) {
	service := NewAuditService(NewInMemoryAuditStore(), nil)
	for _, eventType := range []AuditEventType{EventSemanticUpdate, EventWorkflowStarted, AuditEventType("custom.unknown")} {
		assert.Equal(t, CategoryCompliance, service.inferCategory(eventType), "event %s", eventType)
	}
}

func TestAuditServiceValidateChainErrorPathsV9(t *testing.T) {
	rangeFail := &v9FailingStore{inner: NewInMemoryAuditStore(), getChainRangeErr: errors.New("v9 range boom")}
	_, err := NewAuditService(rangeFail, nil).ValidateChain(1, 2)
	require.Error(t, err)

	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)
	require.NoError(t, store.Create(&AuditRecord{
		ID:        "bad-hash",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
		Details:   map[string]interface{}{"ch": make(chan int)},
		ChainInfo: ChainInfo{Hash: "h", Sequence: 1},
	}))

	result, err := service.ValidateChain(1, 1)
	require.NoError(t, err)
	require.False(t, result.Valid)
	found := false
	for _, chainErr := range result.Errors {
		if chainErr.Type == "hash_calculation_error" {
			found = true
		}
	}
	assert.True(t, found, "expected hash_calculation_error, got %v", result.Errors)
}

func TestAuditServiceArchiveErrorPathsV9(t *testing.T) {
	exportFail := &v9FailingStore{inner: NewInMemoryAuditStore(), exportErr: errors.New("v9 export boom")}
	_, err := NewAuditService(exportFail, nil).Archive(time.Now(), "/tmp/v9-archive")
	require.Error(t, err)

	deleteFail := &v9FailingStore{inner: NewInMemoryAuditStore(), deleteBeforeErr: errors.New("v9 delete boom")}
	_, err = NewAuditService(deleteFail, nil).Archive(time.Now(), "/tmp/v9-archive")
	require.Error(t, err)
}

func TestFromRecordMarshalErrorsV9(t *testing.T) {
	base := func() *AuditRecord {
		return &AuditRecord{
			ID:        "v9-marshal",
			Timestamp: time.Now().UTC(),
			EventType: EventLogin,
			Outcome:   "success",
			ChainInfo: ChainInfo{Hash: "h", Sequence: 1},
		}
	}

	withBadDetails := base()
	withBadDetails.Details = map[string]interface{}{"ch": make(chan int)}
	_, err := FromRecord(withBadDetails)
	assert.Error(t, err)

	withBadChanges := base()
	withBadChanges.Changes = &ChangeInfo{Before: map[string]interface{}{"ch": make(chan int)}}
	_, err = FromRecord(withBadChanges)
	assert.Error(t, err)
}

func TestDerivePromotedFieldsV9(t *testing.T) {
	gameID, env, functionID, durationMs := derivePromotedFields(nil)
	assert.Equal(t, "", gameID)
	assert.Equal(t, "", env)
	assert.Equal(t, "", functionID)
	assert.Equal(t, int64(0), durationMs)

	record := &AuditRecord{Resource: ResourceInfo{GameID: " g1 ", Environment: " prod "}}
	gameID, env, functionID, durationMs = derivePromotedFields(record)
	assert.Equal(t, "g1", gameID)
	assert.Equal(t, "prod", env)
	assert.Equal(t, "", functionID)
	assert.Equal(t, int64(0), durationMs)

	record = &AuditRecord{Details: map[string]interface{}{
		"game_id":     "dg",
		"env":         "dev",
		"function_id": "fn-1",
		"duration_ms": float64(120),
	}}
	gameID, env, functionID, durationMs = derivePromotedFields(record)
	assert.Equal(t, "dg", gameID)
	assert.Equal(t, "dev", env)
	assert.Equal(t, "fn-1", functionID)
	assert.Equal(t, int64(120), durationMs)

	record = &AuditRecord{Details: map[string]interface{}{"elapsed_ms": float64(55)}}
	_, _, _, durationMs = derivePromotedFields(record)
	assert.Equal(t, int64(55), durationMs)

	record = &AuditRecord{Resource: ResourceInfo{Type: "function", ID: " fn-2 "}}
	_, _, functionID, _ = derivePromotedFields(record)
	assert.Equal(t, "fn-2", functionID)

	record = &AuditRecord{Details: map[string]interface{}{"game_id": 123, "env": 456}}
	gameID, env, _, _ = derivePromotedFields(record)
	assert.Equal(t, "", gameID)
	assert.Equal(t, "", env)
}

func TestDetailsNumericV9(t *testing.T) {
	v, ok := detailsNumeric(int64(5))
	assert.True(t, ok)
	assert.Equal(t, float64(5), v)

	v, ok = detailsNumeric(int(7))
	assert.True(t, ok)
	assert.Equal(t, float64(7), v)

	v, ok = detailsNumeric("not-a-number")
	assert.False(t, ok)
	assert.Equal(t, float64(0), v)

	v, ok = detailsNumeric(float64(1.5))
	assert.True(t, ok)
	assert.Equal(t, float64(1.5), v)
}

func TestNewSQLAuditStoreLoadsLatestCacheV9(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.Create(&AuditModel{
		AuditID:       "v9-latest",
		Timestamp:     time.Now().UTC(),
		EventType:     string(EventLogin),
		Outcome:       "success",
		ChainHash:     "h",
		ChainSequence: 3,
	}).Error)

	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	require.NoError(t, db.Exec("DELETE FROM audit_records").Error)

	latest, err := store.GetLatestRecord()
	require.NoError(t, err)
	assert.Equal(t, "v9-latest", latest.ID)
}

func TestSQLStoreGetFromDatabaseV9(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.Create(&AuditModel{
		AuditID:       "v9-db-only",
		Timestamp:     time.Now().UTC(),
		EventType:     string(EventLogin),
		Outcome:       "success",
		ChainHash:     "h",
		ChainSequence: 7,
	}).Error)

	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	got, err := store.Get("v9-db-only")
	require.NoError(t, err)
	assert.Equal(t, "v9-db-only", got.ID)
}

func TestSQLStoreBackfillPromotedFieldsV9(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.Exec("ALTER TABLE audit_records ADD COLUMN gameId varchar(100)").Error)

	now := time.Now().UTC()
	rows := []AuditModel{
		{AuditID: "v9-bf1", Timestamp: now, EventType: string(EventFunctionInvoke), Outcome: "success", ChainHash: "h1", ChainSequence: 1,
			DetailsJSON: []byte(`{"function_id":"fn-1","game_id":"g1","env":"prod","duration_ms":120}`)},
		{AuditID: "v9-bf2", Timestamp: now, EventType: string(EventPageExecute), Outcome: "success", ChainHash: "h2", ChainSequence: 2,
			ResourceJSON: []byte(`{"type":"function","id":"fn-2"}`)},
		{AuditID: "v9-bf3", Timestamp: now, EventType: string(EventFunctionInvoke), Outcome: "success", ChainHash: "h3", ChainSequence: 3},
		{AuditID: "v9-bf4", Timestamp: now, EventType: string(EventFunctionInvoke), Outcome: "success", ChainHash: "h4", ChainSequence: 4,
			DetailsJSON: []byte("not-json")},
	}
	for i := range rows {
		require.NoError(t, db.Create(&rows[i]).Error)
	}

	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	require.NotNil(t, store)

	var got1 AuditModel
	require.NoError(t, db.Where("audit_id = ?", "v9-bf1").First(&got1).Error)
	assert.Equal(t, "fn-1", got1.FunctionID)
	assert.Equal(t, "g1", got1.GameID)
	assert.Equal(t, "prod", got1.Env)
	assert.Equal(t, int64(120), got1.DurationMs)

	var got2 AuditModel
	require.NoError(t, db.Where("audit_id = ?", "v9-bf2").First(&got2).Error)
	assert.Equal(t, "fn-2", got2.FunctionID)

	var got3 AuditModel
	require.NoError(t, db.Where("audit_id = ?", "v9-bf3").First(&got3).Error)
	assert.Equal(t, "", got3.FunctionID)
}

func TestSQLStoreCreateErrorPathsV9(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	badDetails := &AuditRecord{
		ID:        "v9-bad",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
		Details:   map[string]interface{}{"ch": make(chan int)},
		ChainInfo: ChainInfo{Hash: "h", Sequence: 1},
	}
	assert.Error(t, store.Create(badDetails))

	require.NoError(t, store.Create(&AuditRecord{
		ID:        "v9-dup-1",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
		ChainInfo: ChainInfo{Hash: "h", Sequence: 1},
	}))
	dup := &AuditRecord{
		ID:        "v9-dup-2",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
		ChainInfo: ChainInfo{Hash: "h", Sequence: 1},
	}
	assert.Error(t, store.Create(dup))
}

func TestSQLStoreClosedDBErrorPathsV9(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = store.Get("missing")
	assert.Error(t, err)

	_, err = store.GetLatestRecord()
	assert.Error(t, err)

	_, err = store.GetBySequence(1)
	assert.Error(t, err)

	_, err = store.GetChainRange(1, 2)
	assert.Error(t, err)

	_, _, err = store.List(AuditFilter{}, AuditPage{})
	assert.Error(t, err)

	_, err = store.CountByFilter(AuditFilter{})
	assert.Error(t, err)

	assert.Error(t, store.Delete("missing"))

	_, err = store.Export(AuditFilter{}, "json")
	assert.Error(t, err)
}

func TestSQLStoreListAndChainRangeCorruptJSONV9(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.Create(&AuditModel{
		AuditID:       "v9-corrupt",
		Timestamp:     time.Now().UTC(),
		EventType:     string(EventLogin),
		Outcome:       "success",
		ChainHash:     "h",
		ActorJSON:     []byte("not-json"),
		ChainSequence: 1,
	}).Error)

	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	_, _, err = store.List(AuditFilter{}, AuditPage{PageSize: 10})
	assert.Error(t, err)

	_, err = store.GetChainRange(1, 1)
	assert.Error(t, err)
}

func TestSQLStoreCountByFilterTimeRangeV9(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, store.Create(&AuditRecord{
		ID: "v9-old", Timestamp: now.Add(-2 * time.Hour),
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))
	require.NoError(t, store.Create(&AuditRecord{
		ID: "v9-new", Timestamp: now,
		EventType: EventLogin, Outcome: "success",
		ChainInfo: ChainInfo{Hash: "h2", Sequence: 2},
	}))

	cutoff := now.Add(-1 * time.Hour)
	count, err := store.CountByFilter(AuditFilter{StartTime: &cutoff})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = store.CountByFilter(AuditFilter{EndTime: &cutoff})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = store.CountByFilter(AuditFilter{StartTime: &cutoff, EndTime: &cutoff})
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestSQLStoreExportFormatsV9(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	require.NoError(t, store.Create(&AuditRecord{
		ID:        "v9-export",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
		ChainInfo: ChainInfo{Hash: "h1", Sequence: 1},
	}))

	data, err := store.Export(AuditFilter{}, "jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(data), "v9-export")
	assert.Contains(t, string(data), "\n")

	_, err = store.Export(AuditFilter{}, "xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported export format")
}

func TestInMemoryStoreEdgeCasesV9(t *testing.T) {
	store := NewInMemoryAuditStore()
	for i := 0; i < 3; i++ {
		require.NoError(t, store.Create(&AuditRecord{
			ID: fmt.Sprintf("v9-r%d", i), Timestamp: time.Now().UTC(),
			EventType: EventLogin, Outcome: "success",
		}))
	}

	records, total, err := store.List(AuditFilter{}, AuditPage{Page: 5, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Empty(t, records)

	stats, err := store.GetStats(time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalRecords)
	assert.Empty(t, stats.ByEventType)
	assert.Equal(t, float64(0), stats.FailureRate)
}

func TestAuditWriterErrorPathsV9(t *testing.T) {
	w := NewAuditWriter(v9ErrorWriter{})
	badRecord := &AuditRecord{
		ID:        "v9-bad",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
		Details:   map[string]interface{}{"ch": make(chan int)},
	}
	assert.Error(t, w.Write(badRecord))

	var buf bytes.Buffer
	valid := &AuditRecord{
		ID:        "v9-good",
		Timestamp: time.Now().UTC(),
		EventType: EventLogin,
		Outcome:   "success",
	}
	failingWriter := NewAuditWriter(v9ErrorWriter{})
	assert.Error(t, failingWriter.Write(valid))
	assert.Empty(t, buf.Len())
}
