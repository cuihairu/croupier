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
