package healthprobe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewCheckerICMPFactory(t *testing.T) {
	checker, err := NewChecker(TargetConfig{
		Name:     "gateway",
		Category: TargetCategoryThirdParty,
		Type:     TargetTypeICMP,
		Address:  "127.0.0.1",
		Count:    0,
	})
	if err != nil {
		t.Fatalf("NewChecker() error = %v", err)
	}
	icmp, ok := checker.(*ICMPChecker)
	if !ok {
		t.Fatalf("checker type = %T, want *ICMPChecker", checker)
	}
	if icmp.Count != 1 {
		t.Fatalf("default Count = %d, want 1", icmp.Count)
	}

	if _, err := NewChecker(TargetConfig{Name: "missing", Category: TargetCategoryGame, Type: TargetTypeICMP}); err == nil || !strings.Contains(err.Error(), "icmp requires address") {
		t.Fatalf("missing address error = %v", err)
	}
}

func TestICMPCheckerCapabilityFailureWithoutPanic(t *testing.T) {
	if hasRawICMPCapability(t) {
		t.Skip("environment grants CAP_NET_RAW")
	}
	checker := NewICMPChecker("127.0.0.1", time.Second, 1)
	err := checker.Check(context.Background())
	var capability *CapabilityError
	if !errors.As(err, &capability) || capability.Capability != "CAP_NET_RAW" {
		t.Fatalf("Check() error = %v, want CAP_NET_RAW CapabilityError", err)
	}
}

func TestICMPCheckerLoopbackSuccessWithRawCapability(t *testing.T) {
	if !hasRawICMPCapability(t) {
		t.Skip("environment lacks CAP_NET_RAW")
	}
	checker := NewICMPChecker("127.0.0.1", time.Second, 1)
	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestICMPCheckerInvalidPayloadAddress(t *testing.T) {
	checker := NewICMPChecker("", time.Second, 1)
	err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "configuration: address is required") {
		t.Fatalf("Check() error = %v", err)
	}
}
