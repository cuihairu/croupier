package ipgeo

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestInitDB_AutoDetectV6Config covers the ./configs auto-detection branch for
// the IPv6 BIN: when both env vars are unset and only
// configs/IP2LOCATION-LITE-DB3.IPV6.BIN exists, it is loaded into db6 and used
// for IPv6 lookups.
func TestInitDB_AutoDetectV6Config(t *testing.T) {
	resetDB(t)
	t.Setenv("IP2LOCATION_BIN_PATH", "")
	t.Setenv("IP2LOCATION_BIN_PATH_V6", "")

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Reuse the valid synthetic v6 BIN built by the existing helpers.
	v6, err := os.ReadFile(writeSyntheticV6BIN(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs", "IP2LOCATION-LITE-DB3.IPV6.BIN"), v6, 0o600); err != nil {
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

	if got := Region("2001:db8::1"); got != "United States" {
		t.Fatalf("v6 autodetect lookup = %q, want %q", got, "United States")
	}
	if db6 == nil {
		t.Fatal("db6 should be auto-detected from ./configs")
	}
}

// TestRegion_GetAllError covers the db.Get_all error branch: a BIN whose
// 64-byte header is valid (so OpenDB succeeds) but whose record section is
// truncated makes the first binary-search read fail with EOF, so Region
// degrades to "" instead of panicking.
func TestRegion_GetAllError(t *testing.T) {
	resetDB(t)

	hdr := make([]byte, 64)
	hdr[0] = 3  // databasetype: DB3
	hdr[1] = 4  // databasecolumn
	hdr[2] = 20 // year 2020: bypasses the productcode check
	hdr[3], hdr[4] = 1, 1
	binary.LittleEndian.PutUint32(hdr[5:], 3)  // ipv4 record count
	binary.LittleEndian.PutUint32(hdr[9:], 65) // ipv4 base addr

	path := filepath.Join(t.TempDir(), "truncated.BIN")
	if err := os.WriteFile(path, hdr, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("IP2LOCATION_BIN_PATH", path)
	t.Setenv("IP2LOCATION_BIN_PATH_V6", "")

	if got := Region("1.2.3.4"); got != "" {
		t.Fatalf("Region with truncated BIN = %q, want empty", got)
	}
	if db4 == nil {
		t.Fatal("truncated BIN should still load: its header is valid")
	}
}
