package probe

import (
	"context"
	"testing"
	"time"
)

func TestICMPProbeLoopbackSuccessWithRawCapability(t *testing.T) {
	if !hasRawICMPCapability() {
		t.Skip("environment lacks CAP_NET_RAW")
	}
	result := NewICMPProbe("127.0.0.1", time.Second, 1).Run(context.Background())
	if !result.OK || result.Status != 200 || result.Error != "" {
		t.Fatalf("result = %+v", result)
	}
}
