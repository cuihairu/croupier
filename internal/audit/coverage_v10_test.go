package audit

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ipgeo.Region resolves lazily via sync.Once and reads IP2LOCATION_BIN_PATH at
// its first call. Existing tests already trigger Region through Log with an
// IP, so the only deterministic injection point is process-wide: TestMain
// publishes a synthetic IP2Location DB3 BIN (byte-identical layout to
// internal/ipgeo's own test fixtures) before any test runs. Lookups then hit
// local synthetic data — no network, no real database — which makes Log's
// ipRegion enrichment branch reachable. IPv4 addresses outside the synthetic
// ranges keep the old behavior for other tests only if they map to the JP
// sentinel-covered range; assertions elsewhere never inspect Details, so this
// is assertion-neutral.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "covx-ip2l-")
	code := 0
	if err == nil {
		path := filepath.Join(dir, "IP2LOCATION-LITE-DB3.BIN")
		if werr := os.WriteFile(path, covXSyntheticBIN(), 0o600); werr == nil {
			_ = os.Setenv("IP2LOCATION_BIN_PATH", path)
			_ = os.Setenv("IP2LOCATION_BIN_PATH_V6", "")
		}
	}
	code = m.Run()
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}

// covXWriteString appends a length-prefixed string (IP2Location on-disk
// format) and returns its absolute file offset.
func covXWriteString(buf *bytes.Buffer, s string) uint32 {
	off := uint32(buf.Len())
	buf.WriteByte(byte(len(s)))
	buf.WriteString(s)
	return off
}

// covXSyntheticBIN builds a minimal valid IP2Location DB3 BIN with three IPv4
// records plus sentinel:
//
//	1.2.3.4 -> United States/California/Mountain View
//	2.2.2.2 -> China
//	3.3.3.3 -> JP (short-name fallback)
func covXSyntheticBIN() []byte {
	buf := &bytes.Buffer{}

	hdr := make([]byte, 64)
	hdr[0] = 3 // databasetype: DB3
	hdr[1] = 4 // databasecolumn
	hdr[2] = 20
	hdr[3], hdr[4] = 1, 1
	binary.LittleEndian.PutUint32(hdr[5:], 3)  // ipv4 record count
	binary.LittleEndian.PutUint32(hdr[9:], 65) // ipv4 base addr
	buf.Write(hdr)

	const strBase = 64 + 4*16
	strs := &bytes.Buffer{}
	cUS := strBase + uint32(strs.Len())
	covXWriteString(strs, "US")
	covXWriteString(strs, "United States")
	rUS := strBase + uint32(strs.Len())
	covXWriteString(strs, "California")
	cyUS := strBase + uint32(strs.Len())
	covXWriteString(strs, "Mountain View")
	cCN := strBase + uint32(strs.Len())
	covXWriteString(strs, "CN")
	covXWriteString(strs, "China")
	rEmpty := strBase + uint32(strs.Len())
	covXWriteString(strs, "")
	cyEmpty := strBase + uint32(strs.Len())
	covXWriteString(strs, "")
	cJP := strBase + uint32(strs.Len())
	covXWriteString(strs, "JP")
	covXWriteString(strs, "")

	writeRec := func(from uint32, c, r, cy uint32) {
		var rec [16]byte
		binary.LittleEndian.PutUint32(rec[0:], from)
		binary.LittleEndian.PutUint32(rec[4:], c)
		binary.LittleEndian.PutUint32(rec[8:], r)
		binary.LittleEndian.PutUint32(rec[12:], cy)
		buf.Write(rec[:])
	}
	writeRec(0x01020304, cUS, rUS, cyUS)
	writeRec(0x02020202, cCN, rEmpty, cyEmpty)
	writeRec(0x03030303, cJP, rEmpty, cyEmpty)
	writeRec(0xFFFFFFFF, 0, 0, 0)
	buf.Write(strs.Bytes())
	return buf.Bytes()
}

// TestCovXLogIPRegion covers audit.go Log: the ipRegion enrichment when
// ipgeo.Region resolves a non-empty label (synthetic BIN loaded via TestMain),
// plus the preexisting-ipRegion pass-through.
func TestCovXLogIPRegion(t *testing.T) {
	store := NewInMemoryAuditStore()
	service := NewAuditService(store, nil)

	rec, err := service.Log(context.Background(), EventLogin,
		WithIPAddress("1.2.3.4", "covx-agent"))
	require.NoError(t, err)
	require.Equal(t, "1.2.3.4", rec.Actor.IPAddress)
	require.Equal(t, "United States/California/Mountain View", rec.Details["ipRegion"])

	rec2, err := service.Log(context.Background(), EventLogin,
		WithIPAddress("1.2.3.4", "covx-agent"),
		WithDetails(map[string]interface{}{"ipRegion": "custom"}),
	)
	require.NoError(t, err)
	require.Equal(t, "custom", rec2.Details["ipRegion"])
}

// TestCovXBackfillUpdateError covers store.go backfillPromotedFields: the
// batched UPDATE failure branch. A legacy function.invoke row with derivable
// function_id is inserted first, then an Update callback forces every UPDATE
// to error, so the constructor's backfill aborts gracefully (return, no
// panic).
func TestCovXBackfillUpdateError(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.Create(&AuditModel{
		AuditID:       "covx-bf",
		Timestamp:     time.Now().UTC(),
		EventType:     string(EventFunctionInvoke),
		Outcome:       "success",
		ChainHash:     "h",
		ChainSequence: 1,
		DetailsJSON:   []byte(`{"function_id":"fn-covx","duration_ms":42}`),
	}).Error)
	require.NoError(t, db.Callback().Update().Before("gorm:update").
		Register("covx:update_fail", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("covx update boom"))
		}))

	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	require.NotNil(t, store)
}

// TestCovXGetLatestRecordFromDB covers store.go GetLatestRecord: the
// cache-miss DB query success path (First -> ToRecord). The row is inserted
// straight through GORM so the store's memCache stays empty.
func TestCovXGetLatestRecordFromDB(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	require.NoError(t, db.Create(&AuditModel{
		AuditID:       "covx-db-latest",
		Timestamp:     time.Now().UTC(),
		EventType:     string(EventLogin),
		Outcome:       "success",
		ChainHash:     "h",
		ChainSequence: 9,
	}).Error)

	got, err := store.GetLatestRecord()
	require.NoError(t, err)
	require.Equal(t, "covx-db-latest", got.ID)
	require.Equal(t, int64(9), got.ChainInfo.Sequence)
}
