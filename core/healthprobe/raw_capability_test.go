package healthprobe

import (
	"net"
	"os"
	"testing"
)

func hasRawICMPCapability(t *testing.T) bool {
	t.Helper()
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return false
	}
	_ = conn.Close()
	return os.Geteuid() == 0
}
