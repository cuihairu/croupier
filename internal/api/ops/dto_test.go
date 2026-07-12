package ops

import (
	"encoding/json"
	"testing"
)

func TestOpsAgentInfoJSONIncludesAddr(t *testing.T) {
	agent := OpsAgentInfo{
		AgentID: "agent-1",
		Addr:    "127.0.0.1:19091",
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("marshal ops agent info: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal ops agent info json: %v", err)
	}
	if got["addr"] != "127.0.0.1:19091" {
		t.Fatalf("addr = %v, want %q", got["addr"], "127.0.0.1:19091")
	}
}
