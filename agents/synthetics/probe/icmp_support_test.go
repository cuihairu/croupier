package probe

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func hasRawICMPCapability() bool {
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return false
	}
	_ = conn.Close()
	return os.Geteuid() == 0
}

func TestICMPProbeExplicitCapabilityFailureWithoutPanic(t *testing.T) {
	if hasRawICMPCapability() {
		t.Skip("environment grants CAP_NET_RAW")
	}
	result := NewICMPProbe("127.0.0.1", time.Second, 1).Run(context.Background())
	if result.OK || !strings.Contains(result.Error, "capability: CAP_NET_RAW is required for ICMP") {
		t.Fatalf("result = %+v, want CAP_NET_RAW capability error", result)
	}
}
