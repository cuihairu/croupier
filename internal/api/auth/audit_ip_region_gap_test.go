// Coverage target: the ipRegion branch of Service.recordLoginAudit — when
// req.ClientIP is non-empty and ipgeo.Region hits, the region is written into
// metadata["ipRegion"] (service.go:580).
//
// ipgeo initializes lazily via sync.Once, and the environment variables are read
// once, on the first Region call; under -shuffle=on the case order is not
// deterministic and filename sorting does not guarantee "this test's BIN is the
// one the Once sees first" (observed: another case consumes the Once and this
// test then gets an empty region). So right after setting the env this test
// calls ipgeo.ResetForTesting to replay initDB, and cleanup resets once more
// after restoring the env, letting the other cases re-initialize lazily against
// their own environment. The IPs used by other tests
// (127.0.0.1/10.0.0.1/192.168.1.1) fall on the sentinel record, so Region
// still returns an empty string.
package auth

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/ipgeo"
	"github.com/cuihairu/croupier/internal/model"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ipRegionWriteString appends a length-prefixed string (IP2Location disk
// format) and returns its absolute offset in buf.
func ipRegionWriteString(buf *bytes.Buffer, s string) uint32 {
	off := uint32(buf.Len())
	buf.WriteByte(byte(len(s)))
	buf.WriteString(s)
	return off
}

// writeIPRegionSyntheticBIN builds a minimal working IP2Location DB3-format
// IPv4 BIN (isomorphic to the synthetic BIN in internal/ipgeo):
//
//	1.2.3.4         -> US / United States / California / Mountain View
//	2.2.2.2         -> CN / China
//	3.3.3.3         -> JP (short-name fallback)
//	255.255.255.255 -> sentinel upper bound; lookups for other addresses return empty
func writeIPRegionSyntheticBIN(t *testing.T) string {
	t.Helper()
	buf := &bytes.Buffer{}

	hdr := make([]byte, 64)
	hdr[0] = 3  // databasetype: DB3 (country/region/city)
	hdr[1] = 4  // databasecolumn
	hdr[2] = 20 // year 2020: bypasses the productcode check
	hdr[3], hdr[4] = 1, 1
	binary.LittleEndian.PutUint32(hdr[5:], 3)  // ipv4 record count
	binary.LittleEndian.PutUint32(hdr[9:], 65) // ipv4 base addr (1-based 65 -> offset 64)
	buf.Write(hdr)

	// Four record slots (16 bytes each): 3 data records + 1 sentinel; the string
	// section follows immediately.
	const strBase = 64 + 4*16
	strs := &bytes.Buffer{}
	cUS := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "US")
	// The long country name must follow the short one: the library reads at
	// country_ptr+3 (+3 skips the short name).
	ipRegionWriteString(strs, "United States")
	rUS := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "California")
	cyUS := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "Mountain View")
	cCN := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "CN")
	ipRegionWriteString(strs, "China")
	rEmpty := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "")
	cyEmpty := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "")
	cJP := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "JP")
	ipRegionWriteString(strs, "")

	writeRec := func(from uint32, c, r, cy uint32) {
		var rec [16]byte
		binary.LittleEndian.PutUint32(rec[0:], from)
		binary.LittleEndian.PutUint32(rec[4:], c)
		binary.LittleEndian.PutUint32(rec[8:], r)
		binary.LittleEndian.PutUint32(rec[12:], cy)
		buf.Write(rec[:])
	}
	writeRec(0x01020304, cUS, rUS, cyUS)       // 1.2.3.4
	writeRec(0x02020202, cCN, rEmpty, cyEmpty) // 2.2.2.2
	writeRec(0x03030303, cJP, rEmpty, cyEmpty) // 3.3.3.3
	writeRec(0xFFFFFFFF, 0, 0, 0)              // sentinel upper bound

	buf.Write(strs.Bytes())

	path := filepath.Join(t.TempDir(), "IP2LOCATION-LITE-DB3.BIN")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

func TestRecordLoginAudit_IPRegionFromSyntheticDB(t *testing.T) {
	prev, hadPrev := os.LookupEnv("IP2LOCATION_BIN_PATH")
	require.NoError(t, os.Setenv("IP2LOCATION_BIN_PATH", writeIPRegionSyntheticBIN(t)))
	// The Once may already have been consumed by another case in this package
	// (earlier in shuffle order) with the default environment: replaying initDB
	// guarantees the synthetic BIN takes effect.
	ipgeo.ResetForTesting()
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv("IP2LOCATION_BIN_PATH", prev)
		} else {
			_ = os.Unsetenv("IP2LOCATION_BIN_PATH")
		}
		ipgeo.ResetForTesting()
	})

	db := setupTestDB(t)
	svc := withTableAudit(t, db, NewService(
		model.NewAdminModel(db),
		permissionservice.NewPermissionService(db),
		"secret",
	))

	svc.recordLoginAudit(context.Background(), "region-user", "auth.login", "success", &LoginRequest{
		ClientIP:  "1.2.3.4",
		UserAgent: "ua-region",
	}, "", "")

	row := lastAuditRow(t, db)
	assert.Contains(t, row["details_json"], "ipRegion")
	assert.Contains(t, row["details_json"], "United States")
}
