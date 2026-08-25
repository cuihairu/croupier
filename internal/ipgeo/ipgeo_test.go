package ipgeo

import (
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
