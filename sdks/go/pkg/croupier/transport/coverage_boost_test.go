package transport

import "testing"

func TestBuildDialAddrs_ExplicitAddressesTrimAndNormalize(t *testing.T) {
	cfg := &Config{
		Insecure:  true,
		Addresses: []string{"  a:1  ", "", "b:2", "  "},
	}
	addrs := buildDialAddrs(cfg)
	if len(addrs) != 2 {
		t.Fatalf("expected 2 trimmed addresses, got %v", addrs)
	}
	if addrs[0] != "tcp://a:1" || addrs[1] != "tcp://b:2" {
		t.Fatalf("unexpected normalized addrs %v", addrs)
	}
}

func TestBuildDialAddrs_ExplicitAddressesTLS(t *testing.T) {
	cfg := &Config{Addresses: []string{"a:1"}}
	addrs := buildDialAddrs(cfg)
	if len(addrs) != 1 || addrs[0] != "tls+tcp://a:1" {
		t.Fatalf("secure default should add tls+tcp://, got %v", addrs)
	}
}

func TestBuildDialAddrs_CommaSeparatedAddress(t *testing.T) {
	cfg := &Config{Insecure: true, Address: "h1:1, h2:2,,h3:3"}
	addrs := buildDialAddrs(cfg)
	if len(addrs) != 3 {
		t.Fatalf("expected 3 addresses from comma list, got %v", addrs)
	}
	want := []string{"tcp://h1:1", "tcp://h2:2", "tcp://h3:3"}
	for i := range want {
		if addrs[i] != want[i] {
			t.Fatalf("addr[%d] = %q, want %q", i, addrs[i], want[i])
		}
	}
}

func TestBuildDialAddrs_IPCGetsSchemePrefix(t *testing.T) {
	cfg := &Config{Insecure: true, IPCAddress: "croupier-agent.sock"}
	addrs := buildDialAddrs(cfg)
	if len(addrs) == 0 || addrs[0] != "ipc://croupier-agent.sock" {
		t.Fatalf("IPC without scheme should be prefixed, got %v", addrs)
	}
}

func TestBuildDialAddrs_IPCDuplicateOfAddressSkipped(t *testing.T) {
	cfg := &Config{
		Insecure:   true,
		IPCAddress: "ipc:///tmp/sock",
		Address:    "ipc:///tmp/sock,tcp-host:9",
	}
	addrs := buildDialAddrs(cfg)
	if len(addrs) != 2 {
		t.Fatalf("duplicate IPC entry should be skipped, got %v", addrs)
	}
	if addrs[0] != "ipc:///tmp/sock" || addrs[1] != "tcp://tcp-host:9" {
		t.Fatalf("unexpected addrs %v", addrs)
	}
}

func TestBuildDialAddrs_EmptyEverythingFallsBackToDefault(t *testing.T) {
	addrs := buildDialAddrs(&Config{})
	if len(addrs) != 1 || addrs[0] != "tcp://127.0.0.1:19091" {
		t.Fatalf("default fallback expected, got %v", addrs)
	}
}

func TestBuildDialAddrs_AddressesTakePrecedenceOverIPCAndAddress(t *testing.T) {
	cfg := &Config{
		Insecure:   true,
		Addresses:  []string{"explicit:1"},
		IPCAddress: "ignored.sock",
		Address:    "ignored:2",
	}
	addrs := buildDialAddrs(cfg)
	if len(addrs) != 1 || addrs[0] != "tcp://explicit:1" {
		t.Fatalf("explicit Addresses should win, got %v", addrs)
	}
}

func TestNormalizeAddress_ExistingSchemesKept(t *testing.T) {
	for _, scheme := range []string{"inproc", "ipc", "tcp", "tls+tcp", "ws", "wss"} {
		addr := scheme + "://host:1"
		if got := normalizeAddress(addr, true); got != addr {
			t.Fatalf("scheme %q should be kept, got %q", scheme, got)
		}
	}
}

func TestNormalizeAddress_BareHostDependsOnInsecure(t *testing.T) {
	if got := normalizeAddress("h:1", true); got != "tcp://h:1" {
		t.Fatalf("insecure bare host = %q", got)
	}
	if got := normalizeAddress("h:1", false); got != "tls+tcp://h:1" {
		t.Fatalf("secure bare host = %q", got)
	}
}

func TestDialAddr_AllAddressesBlankFallsBackToFirst(t *testing.T) {
	cfg := &Config{Insecure: true, Addresses: []string{"  ", ""}}
	if got := dialAddr(cfg); got != "tcp://127.0.0.1:19091" {
		t.Fatalf("blank addresses should fall back to default, got %q", got)
	}
}
