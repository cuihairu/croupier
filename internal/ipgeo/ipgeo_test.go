package ipgeo

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// resetDB clears the package-level databases and the init-once gate so tests
// can exercise both the "no BIN" degradation and a controlled env probe.
func resetDB(t *testing.T) {
	t.Helper()
	old4, old6 := db4, db6
	t.Cleanup(func() {
		db4, db6 = old4, old6
		once = sync.Once{}
	})
	db4, db6 = nil, nil
	once = sync.Once{}
}

func TestRegion_NoDatabase(t *testing.T) {
	resetDB(t)
	t.Setenv("IP2LOCATION_BIN_PATH", "")
	t.Setenv("IP2LOCATION_BIN_PATH_V6", "")
	// No BIN configured: graceful degradation, empty region.
	if got := Region("8.8.8.8"); got != "" {
		t.Fatalf("Region with no DB = %q, want empty", got)
	}
}

func TestRegion_InvalidIP(t *testing.T) {
	resetDB(t)
	for _, ip := range []string{"", "not-an-ip", "999.999.999.999", "  "} {
		if got := Region(ip); got != "" {
			t.Fatalf("Region(%q) = %q, want empty", ip, got)
		}
	}
}

func TestInitDB_MissingFiles(t *testing.T) {
	resetDB(t)
	t.Setenv("IP2LOCATION_BIN_PATH", "/nonexistent/ipdb.BIN")
	t.Setenv("IP2LOCATION_BIN_PATH_V6", "")
	// Missing file must not panic; lookups stay empty.
	if got := Region("1.2.3.4"); got != "" {
		t.Fatalf("Region with missing DB = %q", got)
	}
	if db4 != nil {
		t.Fatal("db4 should stay nil for missing file")
	}
}

func TestInitDB_IPv6Heuristic(t *testing.T) {
	resetDB(t)
	// Only p4 set but filename contains "ipv6" → routed to the v6 slot.
	t.Setenv("IP2LOCATION_BIN_PATH", "/nonexistent/IP2LOCATION-LITE-DB3.IPV6.BIN")
	t.Setenv("IP2LOCATION_BIN_PATH_V6", "")
	Region("2001:db8::1") // triggers initDB
	if db4 != nil {
		t.Fatal("db4 should stay nil when the only file is IPv6")
	}
}

func TestRegion_ValidIPsNoDB(t *testing.T) {
	resetDB(t)
	t.Setenv("IP2LOCATION_BIN_PATH", "/nonexistent/no-such.BIN")
	t.Setenv("IP2LOCATION_BIN_PATH_V6", "")
	// 合法 IPv4/IPv6、带空白、空串：库不存在时优雅降级为空且不 panic
	if got := Region("8.8.8.8"); got != "" {
		t.Fatalf("ipv4: %q", got)
	}
	if got := Region("2001:4860:4860::8888"); got != "" {
		t.Fatalf("ipv6: %q", got)
	}
	if got := Region("  8.8.8.8  "); got != "" {
		t.Fatalf("whitespace: %q", got)
	}
	if got := Region(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
}

// writeString appends a length-prefixed string (the on-disk format) to buf and
// returns its absolute file offset.
func writeString(buf *bytes.Buffer, s string) uint32 {
	off := uint32(buf.Len())
	buf.WriteByte(byte(len(s)))
	buf.WriteString(s)
	return off
}

// writeSyntheticBIN builds a minimal but valid IP2Location DB3-format BIN:
// databasetype=3 (country/region/city at positions 2/3/4), 4 columns, one IPv4
// record set + sentinel. Records:
//
//	1.2.3.4       -> US / United States / California / Mountain View
//	2.2.2.2       -> CN / China, region+city empty
//	3.3.3.3       -> JP, long name empty (exercises short-name fallback)
//	255.255.255.255 -> sentinel upper bound, never matched
func writeSyntheticBIN(t *testing.T) string {
	t.Helper()
	buf := &bytes.Buffer{}

	hdr := make([]byte, 64)
	hdr[0] = 3  // databasetype: DB3
	hdr[1] = 4  // databasecolumn: ip_from + country + region + city
	hdr[2] = 20 // year 2020: < 21 so the productcode check is bypassed
	hdr[3], hdr[4] = 1, 1
	binary.LittleEndian.PutUint32(hdr[5:], 3)  // ipv4 record count
	binary.LittleEndian.PutUint32(hdr[9:], 65) // ipv4 base addr (1-based pos 65 -> offset 64)
	buf.Write(hdr)

	// Reserve 4 record slots (16 bytes each): 3 data + 1 sentinel. The string
	// section starts after them, offsets computed before writing records.
	const strBase = 64 + 4*16
	strings := &bytes.Buffer{}
	cUS := strBase + uint32(strings.Len())
	writeString(strings, "US")
	// long country name must sit right after the short one: the library reads
	// it at country_ptr+3 (the "US" + separator bytes are skipped by that +3).
	writeString(strings, "United States")
	rUS := strBase + uint32(strings.Len())
	writeString(strings, "California")
	cyUS := strBase + uint32(strings.Len())
	writeString(strings, "Mountain View")
	cCN := strBase + uint32(strings.Len())
	writeString(strings, "CN")
	writeString(strings, "China")
	rEmpty := strBase + uint32(strings.Len())
	writeString(strings, "")
	cyEmpty := strBase + uint32(strings.Len())
	writeString(strings, "")
	cJP := strBase + uint32(strings.Len())
	writeString(strings, "JP")
	writeString(strings, "")
	// (region/city for record 2 reuse rEmpty/cyEmpty)

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
	writeRec(0xFFFFFFFF, 0, 0, 0)              // sentinel

	buf.Write(strings.Bytes())

	path := filepath.Join(t.TempDir(), "IP2LOCATION-LITE-DB3.BIN")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write synthetic BIN: %v", err)
	}
	return path
}

func TestRegion_WithSyntheticDB(t *testing.T) {
	resetDB(t)
	t.Setenv("IP2LOCATION_BIN_PATH", writeSyntheticBIN(t))
	t.Setenv("IP2LOCATION_BIN_PATH_V6", "")

	Region("1.2.3.4") // trigger initDB
	if db4 == nil {
		t.Fatal("synthetic DB should be loaded into db4")
	}

	// Full record: country long / region / city joined.
	if got := Region("1.2.3.4"); got != "United States/California/Mountain View" {
		t.Fatalf("full record = %q", got)
	}
	// Region+city empty: only the country remains, no dangling separators.
	if got := Region("2.2.2.2"); got != "China" {
		t.Fatalf("country-only record = %q", got)
	}
	// Long country name empty: falls back to the short code.
	if got := Region("3.3.3.3"); got != "JP" {
		t.Fatalf("short-name fallback = %q", got)
	}
}

// writeSyntheticV6BIN builds a minimal DB3-format BIN with a single IPv6
// record (2001:db8::/range) to exercise the db6 load + lookup path. IPv6
// numbers are stored big-endian (the library reads them via uint128.FromBytes).
func writeSyntheticV6BIN(t *testing.T) string {
	t.Helper()
	buf := &bytes.Buffer{}

	hdr := make([]byte, 64)
	hdr[0], hdr[1] = 3, 4
	hdr[2] = 20
	hdr[3], hdr[4] = 1, 1
	binary.LittleEndian.PutUint32(hdr[13:], 1)  // ipv6 record count
	binary.LittleEndian.PutUint32(hdr[17:], 65) // ipv6 base addr (1-based pos -> offset 64)
	buf.Write(hdr)

	// Record section: one 28-byte v6 record + a 16-byte sentinel ip_from.
	strBase := uint32(64 + 28 + 16)
	strings := &bytes.Buffer{}
	cUS := strBase + uint32(strings.Len())
	writeString(strings, "US")
	writeString(strings, "United States")
	rEmpty := strBase + uint32(strings.Len())
	writeString(strings, "")
	cyEmpty := strBase + uint32(strings.Len())
	writeString(strings, "")

	rec := &bytes.Buffer{}
	rec.Write([]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}) // 2001:db8::
	var ptrs [12]byte
	binary.LittleEndian.PutUint32(ptrs[0:], cUS)
	binary.LittleEndian.PutUint32(ptrs[4:], rEmpty)
	binary.LittleEndian.PutUint32(ptrs[8:], cyEmpty)
	rec.Write(ptrs[:])
	// sentinel upper bound: 2001:db8::ffff:ffff (exactly 16 bytes)
	rec.Write([]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff})
	buf.Write(rec.Bytes())

	buf.Write(strings.Bytes())

	path := filepath.Join(t.TempDir(), "IP2LOCATION-LITE-DB3.IPV6.BIN")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write synthetic v6 BIN: %v", err)
	}
	return path
}

func TestRegion_WithSyntheticV6DB(t *testing.T) {
	resetDB(t)
	t.Setenv("IP2LOCATION_BIN_PATH", "")
	t.Setenv("IP2LOCATION_BIN_PATH_V6", writeSyntheticV6BIN(t))

	if got := Region("2001:db8::1"); got != "United States" {
		t.Fatalf("v6 lookup = %q", got)
	}
	if db6 == nil {
		t.Fatal("db6 should be loaded from the v6 BIN")
	}
}

func TestInitDB_AutoDetectConfigsDir(t *testing.T) {
	resetDB(t)
	t.Setenv("IP2LOCATION_BIN_PATH", "")
	t.Setenv("IP2LOCATION_BIN_PATH_V6", "")

	// env 为空时回退到 ./configs 自动探测：文件存在但内容非法 → 打开失败、
	// 保持 nil 且不 panic（覆盖 os.Stat 探测与 OpenDB 失败分支）。
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs", "IP2LOCATION-LITE-DB3.BIN"), []byte("not a real bin"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if got := Region("8.8.8.8"); got != "" {
		t.Fatalf("invalid BIN should degrade to empty, got %q", got)
	}
	if db4 != nil {
		t.Fatal("db4 should stay nil after failed OpenDB")
	}
}
