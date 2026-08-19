package protocol

import (
	"strings"
	"testing"

	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
)

func TestMsgIDString_AllKnownMessages(t *testing.T) {
	tests := []struct {
		msgID    uint32
		expected string
	}{
		{MsgRegisterCapabilitiesReq, "RegisterCapabilitiesRequest"},
		{MsgRegisterCapabilitiesResp, "RegisterCapabilitiesResponse"},
		{MsgClientHeartbeatRequest, "ClientHeartbeatRequest"},
		{MsgClientHeartbeatResponse, "ClientHeartbeatResponse"},
		{MsgListClientsRequest, "ListClientsRequest"},
		{MsgListClientsResponse, "ListClientsResponse"},
		{MsgStartTaskRequest, "StartTaskRequest"},
		{MsgStartTaskResponse, "StartTaskResponse"},
		{MsgStreamTaskRequest, "StreamTaskRequest"},
		{MsgCancelTaskRequest, "CancelTaskRequest"},
		{MsgCancelTaskResponse, "CancelTaskResponse"},
		{MsgProviderConnectRequest, "ProviderConnectRequest"},
		{MsgProviderConnectResponse, "ProviderConnectResponse"},
		{MsgProviderHeartbeatRequest, "ProviderHeartbeatRequest"},
		{MsgProviderHeartbeatResponse, "ProviderHeartbeatResponse"},
		{MsgProviderDrainRequest, "ProviderDrainRequest"},
		{MsgProviderDrainResponse, "ProviderDrainResponse"},
		{MsgGetTaskResultRequest, "GetTaskResultRequest"},
		{MsgGetTaskResultResponse, "GetTaskResultResponse"},
	}
	for _, tt := range tests {
		if got := MsgIDString(tt.msgID); got != tt.expected {
			t.Errorf("MsgIDString(%#x) = %q, want %q", tt.msgID, got, tt.expected)
		}
	}
}

func TestDebugStringForBody_Branches(t *testing.T) {
	// nil message with short body → hex.
	out := DebugStringForBody(MsgInvokeRequest, 7, []byte{0xde, 0xad}, nil)
	if !strings.Contains(out, "dead") {
		t.Fatalf("short hex body missing: %s", out)
	}

	// Long body → truncated hex.
	long := make([]byte, 64)
	for i := range long {
		long[i] = byte(i)
	}
	out = DebugStringForBody(MsgInvokeRequest, 7, long, nil)
	if !strings.Contains(out, "... (64 bytes)") {
		t.Fatalf("truncated body missing: %s", out)
	}

	// Empty body keeps the placeholder.
	out = DebugStringForBody(MsgInvokeRequest, 7, nil, nil)
	if !strings.Contains(out, "<body>") {
		t.Fatalf("placeholder missing: %s", out)
	}

	// Valid proto message renders as JSON.
	out = DebugStringForBody(MsgInvokeRequest, 7, nil, &sdkv1.InvokeRequest{FunctionId: "f"})
	if !strings.Contains(out, `"f"`) {
		t.Fatalf("proto JSON missing: %s", out)
	}
}
