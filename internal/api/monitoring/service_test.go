package monitoring

import "testing"

func TestNormalizeAgentSnapshotPrefersAddrAndKeepsRPCAddrAlias(t *testing.T) {
	item := map[string]interface{}{
		"id":       "agent-1",
		"agent_id": "agent-1",
		"game_id":  "game-1",
		"env":      "prod",
		"addr":     "127.0.0.1:19091",
		"rpc_addr": "127.0.0.1:19091",
	}

	got := normalizeAgentSnapshot(item)
	if got["addr"] != "127.0.0.1:19091" {
		t.Fatalf("addr = %v, want %q", got["addr"], "127.0.0.1:19091")
	}
	if got["rpcAddr"] != "127.0.0.1:19091" {
		t.Fatalf("rpcAddr = %v, want %q", got["rpcAddr"], "127.0.0.1:19091")
	}
}
