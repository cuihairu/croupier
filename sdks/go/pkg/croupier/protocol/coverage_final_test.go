package protocol

// Final coverage boost: control-request lane classification, remaining
// MsgIDString labels and the debug-body JSON error branch.

import (
	"math"
	"strings"
	"testing"

	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestIsControlRequestMatrix(t *testing.T) {
	control := []uint32{
		MsgRegisterRequest,
		MsgHeartbeatRequest,
		MsgRegisterCapabilitiesReq,
		MsgRegisterClientRequest,
		MsgClientHeartbeatRequest,
		MsgProviderConnectRequest,
		MsgProviderHeartbeatRequest,
		MsgProviderDrainRequest,
	}
	for _, id := range control {
		if !IsControlRequest(id) {
			t.Fatalf("expected %s (0x%06X) to be a control request", MsgIDString(id), id)
		}
	}

	business := []uint32{
		MsgInvokeRequest,
		MsgStartTaskRequest,
		MsgStreamTaskRequest,
		MsgCancelTaskRequest,
		MsgProviderFilePushRequest,
		MsgProviderDrainResponse,
		MsgHeartbeatResponse,
		0x099999,
	}
	for _, id := range business {
		if IsControlRequest(id) {
			t.Fatalf("expected %s (0x%06X) to stay on the business lane", MsgIDString(id), id)
		}
	}
}

func TestMsgIDStringRemainingLabels(t *testing.T) {
	cases := map[uint32]string{
		MsgHeartbeatResponse:      "HeartbeatResponse",
		MsgRegisterClientRequest:  "RegisterClientRequest",
		MsgRegisterClientResponse: "RegisterClientResponse",
	}
	for id, want := range cases {
		if got := MsgIDString(id); got != want {
			t.Fatalf("MsgIDString(0x%06X) = %q, want %q", id, got, want)
		}
	}
	// spot-check a couple of neighbours to guard against regressions
	if MsgIDString(MsgRegisterCapabilitiesReq) != "RegisterCapabilitiesRequest" {
		t.Fatal("RegisterCapabilitiesRequest label drifted")
	}
	if MsgIDString(MsgProviderFilePushRequest) != "Unknown(0x050109)" {
		t.Fatalf("file push keeps its hex label, got %q", MsgIDString(MsgProviderFilePushRequest))
	}
}

func TestDebugStringForBodyJSONError(t *testing.T) {
	// An out-of-range well-known Duration is refused by protojson marshal.
	bad := &durationpb.Duration{Seconds: math.MaxInt64}
	out := DebugStringForBody(MsgInvokeRequest, 42, nil, bad)
	if !strings.Contains(out, "<json error:") {
		t.Fatalf("expected JSON error marker, got %q", out)
	}

	// valid message still renders JSON
	good := &sdkv1.InvokeRequest{FunctionId: "fn"}
	out = DebugStringForBody(MsgInvokeRequest, 1, nil, good)
	if !strings.Contains(out, `"functionId"`) && !strings.Contains(out, "fn") {
		t.Fatalf("expected marshaled body, got %q", out)
	}

	// long raw body is truncated
	long := make([]byte, 64)
	for i := range long {
		long[i] = byte(i)
	}
	out = DebugStringForBody(MsgInvokeRequest, 1, long, nil)
	if !strings.Contains(out, "... (64 bytes)") {
		t.Fatalf("expected truncation marker, got %q", out)
	}
}
