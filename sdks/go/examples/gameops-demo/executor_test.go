package main

import (
	"encoding/json"
	"testing"
)

func listResult(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGameOpsListFilterPaginationSort(t *testing.T) {
	store := newGameOpsStore()

	// 筛选：status=banned 只回 3 名（26 人中 %7=0 → 20007/20014/20021）。
	raw, _ := store.playerList(contextTODO(), mustJSON(t, map[string]any{
		"current": 1, "pageSize": 100, "status": "banned", "orderBy": "id", "orderDirection": "asc",
	}))
	res := listResult(t, raw)
	items := res["items"].([]any)
	if res["total"].(float64) != 3 || len(items) != 3 {
		t.Fatalf("banned filter total=%v len=%d", res["total"], len(items))
	}

	// 分页：pageSize=10 → page2 起于 index 10。
	raw, _ = store.playerList(contextTODO(), mustJSON(t, map[string]any{"current": 2, "pageSize": 10}))
	res = listResult(t, raw)
	if res["total"].(float64) != 26 || len(res["items"].([]any)) != 10 {
		t.Fatalf("pagination total=%v len=%d", res["total"], len(res["items"].([]any)))
	}

	// 依赖排序语义：orderBy=level + desc → 首行为最大值。
	allPlayers := storeNotLockedPlayers(store)
	maxLevel := 0
	for _, p := range allPlayers {
		if p.Level > maxLevel {
			maxLevel = p.Level
		}
	}
	raw, _ = store.playerList(contextTODO(), mustJSON(t, map[string]any{
		"current": 1, "pageSize": 100, "orderBy": "level", "orderDirection": "desc",
	}))
	res = listResult(t, raw)
	first := res["items"].([]any)[0].(map[string]any)
	if int(first["level"].(float64)) != maxLevel {
		t.Fatalf("sort desc first.level=%v want %d", first["level"], maxLevel)
	}
}

func storeNotLockedPlayers(s *gameOpsStore) []gameOpsPlayer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]gameOpsPlayer, len(s.players))
	copy(out, s.players)
	return out
}
