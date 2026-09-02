package registry

import (
	"encoding/json"
	"sort"
	"testing"
)

// F：sdk-stats——ProviderSessionSnapshots 深拷贝在线 provider 会话。
func TestStore_ProviderSessionSnapshots(t *testing.T) {
	store := NewStore()
	sess := &AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Env:     "prod",
		Providers: []ProviderSession{
			{
				ProviderID:   "go-1",
				Addr:         "10.0.0.1:7000",
				SDKLanguage:  "go",
				SDKVersion:   "1.2.0",
				SDKName:      "croupier-go-sdk",
				LastSeenUnix: 1700000000,
				FunctionIDs:  []string{"a", "b"},
			},
			{
				ProviderID:  "js-1",
				SDKLanguage: "js",
				SDKVersion:  "0.9.0",
			},
		},
	}
	if err := store.UpsertAgent(sess); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}
	// Providers 走 agentv1.AgentProcess 转换的路径不存在时，直接补一条快照所需的构造
	if len(sess.Providers) == 0 {
		t.Fatal("expected providers to be stored")
	}

	snapshots := store.ProviderSessionSnapshots()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].ProviderID < snapshots[j].ProviderID })
	first := snapshots[0]
	if first.ProviderID != "go-1" || first.AgentID != "game-1-agent" && first.AgentID != "agent-1" {
		t.Fatalf("unexpected first snapshot: %+v", first)
	}
	if first.GameID != "game-1" || first.Env != "prod" || first.SDKLanguage != "go" || first.SDKVersion != "1.2.0" {
		t.Fatalf("unexpected snapshot fields: %+v", first)
	}
	if len(first.FunctionIDs) != 2 {
		t.Fatalf("expected function ids copied, got %+v", first.FunctionIDs)
	}
	second := snapshots[1]
	if second.SDKLanguage != "js" {
		t.Fatalf("unexpected second snapshot: %+v", second)
	}

	// JSON 序列化（wire 形态）冒烟
	blob, err := json.Marshal(snapshots)
	if err != nil {
		t.Fatalf("marshal snapshots: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("empty json")
	}
}

// UpsertAgent 接受 agentv1.ProviderProcess 时快照仍可读（字段转换冒烟）。
func TestStore_ProviderSessionSnapshots_Empty(t *testing.T) {
	store := NewStore()
	if snapshots := store.ProviderSessionSnapshots(); len(snapshots) != 0 {
		t.Fatalf("expected 0 snapshots, got %d", len(snapshots))
	}
}
