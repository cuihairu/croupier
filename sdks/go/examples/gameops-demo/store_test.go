package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGameOpsStoreSeedInvariants(t *testing.T) {
	s := newGameOpsStore()
	if len(s.servers) != 3 {
		t.Fatalf("servers = %d, want 3", len(s.servers))
	}
	if len(s.players) != 26 {
		t.Fatalf("players = %d, want 26", len(s.players))
	}
	if len(s.tasks) != 4 {
		t.Fatalf("tasks = %d, want 4", len(s.tasks))
	}
	if len(s.bans) != 12 {
		t.Fatalf("bans = %d, want 12", len(s.bans))
	}
	if len(s.orders) != 30 {
		t.Fatalf("orders = %d, want 30", len(s.orders))
	}
	if len(s.activities) != 8 {
		t.Fatalf("activities = %d, want 8", len(s.activities))
	}

	longName := false
	bannedIDs := make(map[string]bool)
	for _, p := range s.players {
		if len(p.Nickname) > 40 {
			longName = true
		}
		if p.Status == "banned" {
			bannedIDs[p.ID] = true
		}
	}
	if !longName {
		t.Fatal("expected a nickname longer than 40 chars (E1 boundary)")
	}

	// 跨页引用一致性：封禁/订单必须指向真实玩家与区服。
	playerByID := make(map[string]gameOpsPlayer)
	for _, p := range s.players {
		playerByID[p.ID] = p
	}
	for _, b := range s.bans {
		p, ok := playerByID[b.PlayerID]
		if !ok {
			t.Fatalf("ban %s references missing player %s", b.ID, b.PlayerID)
		}
		if p.ServerID != b.ServerID {
			t.Fatalf("ban %s server %s != player server %s", b.ID, b.ServerID, p.ServerID)
		}
	}
	for _, o := range s.orders {
		if _, ok := playerByID[o.PlayerID]; !ok {
			t.Fatalf("order %s references missing player %s", o.OrderNo, o.PlayerID)
		}
	}

	// 状态下界齐备：交易对账三态 + 游戏发货齐备。
	r := map[string]bool{}
	for _, o := range s.orders {
		r[o.Reconciliation] = true
	}
	for _, want := range []string{"match", "missing_credit", "missing_order"} {
		if !r[want] {
			t.Fatalf("missing reconciliation state %q", want)
		}
	}
}

func TestGameOpsStoreResetRestoresSeed(t *testing.T) {
	s := newGameOpsStore()
	// mutate
	_, _ = s.playerRecharge(contextTODO(), []byte(`{"playerId":"20001","amount":100,"channel":"wechat"}`))
	_, _ = s.banCreate(contextTODO(), []byte(`{"playerId":"20002","durationHours":24,"reason":"risk"}`))
	s.Reset()
	if len(s.players) != 26 {
		t.Fatalf("players after reset = %d, want 26", len(s.players))
	}
	if len(s.bans) != 12 {
		t.Fatalf("bans after reset = %d, want 12", len(s.bans))
	}
	fresh := newGameOpsStore()
	want := fresh.playerByIDLocked("20001")
	p := s.playerByIDLocked("20001")
	if p == nil || want == nil || p.BalanceGold != want.BalanceGold {
		t.Fatalf("reset did not restore player balance: got %+v want %+v", p, want)
	}
	if strings.HasPrefix(s.bans[0].ID, "ban-demo-") {
		t.Fatalf("reset kept created ban %s", s.bans[0].ID)
	}
}

func TestGameOpsStoreSelectionUpdateVisibleWR(t *testing.T) {
	s := newGameOpsStore()
	raw, err := s.playerBatchBan(contextTODO(), []byte(`{"playerIds":["20001","20004"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result["count"].(float64) != 2 {
		t.Fatalf("batch count = %v", result["count"])
	}
	if s.playerByIDLocked("20001").Status != "banned" {
		t.Fatal("20001 should now be banned (write visible to read)")
	}
}

func TestGameOpsStoreBatchRecharge(t *testing.T) {
	s := newGameOpsStore()
	before1 := s.playerByIDLocked("20001").BalanceGold
	before3 := s.playerByIDLocked("20003").BalanceGold
	raw, err := s.playerBatchRecharge(contextTODO(), []byte(`{"playerIds":["20001","20003"],"amount":100}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result["count"].(float64) != 2 {
		t.Fatalf("batch count = %v", result["count"])
	}
	if result["amount"].(float64) != 100 {
		t.Fatalf("amount = %v", result["amount"])
	}
	if result["total"].(float64) != 200 {
		t.Fatalf("total = %v", result["total"])
	}
	if got := s.playerByIDLocked("20001").BalanceGold; got != before1+100 {
		t.Fatalf("player 20001 balance = %d, want %d", got, before1+100)
	}
	if got := s.playerByIDLocked("20003").BalanceGold; got != before3+100 {
		t.Fatalf("player 20003 balance = %d, want %d", got, before3+100)
	}
	// 负数金额拒绝
	if _, err := s.playerBatchRecharge(contextTODO(), []byte(`{"playerIds":["20001"],"amount":-1}`)); err == nil {
		t.Fatal("negative amount should be rejected")
	}
}
