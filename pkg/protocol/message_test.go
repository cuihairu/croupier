package protocol

import "testing"

func TestProviderSessionMsgIDsUseProviderNames(t *testing.T) {
	tests := []struct {
		msgID uint32
		want  string
	}{
		{MsgProviderConnectRequest, "ProviderConnectRequest"},
		{MsgProviderConnectResponse, "ProviderConnectResponse"},
		{MsgProviderHeartbeatRequest, "ProviderHeartbeatRequest"},
		{MsgProviderHeartbeatResponse, "ProviderHeartbeatResponse"},
		{MsgProviderDrainRequest, "ProviderDrainRequest"},
		{MsgProviderDrainResponse, "ProviderDrainResponse"},
	}

	for _, tt := range tests {
		if got := MsgIDString(tt.msgID); got != tt.want {
			t.Fatalf("MsgIDString(0x%06X) = %q, want %q", tt.msgID, got, tt.want)
		}
	}
}

func TestDeprecatedLocalControlAliasesPointToProviderSessionMsgIDs(t *testing.T) {
	if MsgRegisterLocalRequest != MsgProviderConnectRequest {
		t.Fatalf("MsgRegisterLocalRequest = 0x%06X, want 0x%06X", MsgRegisterLocalRequest, MsgProviderConnectRequest)
	}
	if MsgRegisterLocalResponse != MsgProviderConnectResponse {
		t.Fatalf("MsgRegisterLocalResponse = 0x%06X, want 0x%06X", MsgRegisterLocalResponse, MsgProviderConnectResponse)
	}
	if MsgHeartbeatLocalRequest != MsgProviderHeartbeatRequest {
		t.Fatalf("MsgHeartbeatLocalRequest = 0x%06X, want 0x%06X", MsgHeartbeatLocalRequest, MsgProviderHeartbeatRequest)
	}
	if MsgHeartbeatLocalResponse != MsgProviderHeartbeatResponse {
		t.Fatalf("MsgHeartbeatLocalResponse = 0x%06X, want 0x%06X", MsgHeartbeatLocalResponse, MsgProviderHeartbeatResponse)
	}
}
