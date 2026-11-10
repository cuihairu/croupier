package main

import (
	"context"
	"strings"
	"testing"
)

// Handler-level chains: the exact paths the platform invoke flow exercises.

func TestMailCompensationChainViaHandlers(t *testing.T) {
	s := testStore(t)
	// 1. platform mail.send with attachments
	raw, err := s.mailSend(contextTODO(), mustJSON(t, map[string]any{
		"playerId": "10001", "title": "事故补偿", "content": "ok",
		"attachments": []any{map[string]any{"itemId": "item_ticket", "count": 3}},
	}))
	if err != nil {
		t.Fatalf("mailSend: %v", err)
	}
	mailID := mapResult(t, raw)["mailId"].(string)
	if mailID == "" {
		t.Fatal("empty mailId")
	}

	// 2. mail.list finds it unclaimed
	raw, err = s.mailList(contextTODO(), mustJSON(t, map[string]any{"playerId": "10001", "pageSize": 100}))
	if err != nil {
		t.Fatalf("mailList: %v", err)
	}
	items := mapResult(t, raw)["items"].([]any)
	found := false
	for _, it := range items {
		m := it.(map[string]any)
		if m["id"] == mailID {
			found = true
			if m["claimed"].(bool) || m["expired"].(bool) {
				t.Fatalf("fresh mail state claimed=%v expired=%v", m["claimed"], m["expired"])
			}
		}
	}
	if !found {
		t.Fatalf("mail %s not in list", mailID)
	}

	// 3. claim → attachment into inventory
	raw, err = s.mailClaim(contextTODO(), mustJSON(t, map[string]any{"mailId": mailID}))
	if err != nil {
		t.Fatalf("mailClaim: %v", err)
	}
	if mapResult(t, raw)["success"] != true {
		t.Fatalf("claim result=%s", raw)
	}
	raw, err = s.playerInventory(contextTODO(), mustJSON(t, map[string]any{"playerId": "10001"}))
	if err != nil {
		t.Fatalf("playerInventory: %v", err)
	}
	inv := mapResult(t, raw)
	var tickets float64
	for _, it := range inv["items"].([]any) {
		entry := it.(map[string]any)
		if entry["itemId"] == "item_ticket" {
			tickets = entry["count"].(float64)
		}
	}
	if tickets != 3 {
		t.Fatalf("tickets in inventory=%v want 3", tickets)
	}
}

func TestOrderShipChainViaHandlers(t *testing.T) {
	s := testStore(t)
	before := diamondsOf(t, s, "10001")
	if _, err := s.orderShip(contextTODO(), mustJSON(t, map[string]any{"orderNo": "20261110001"})); err != nil {
		t.Fatalf("ship: %v", err)
	}
	if got := diamondsOf(t, s, "10001"); got != before+6 {
		t.Fatalf("diamonds=%d want %d", got, before+6)
	}
	_, err := s.orderShip(contextTODO(), mustJSON(t, map[string]any{"orderNo": "20261110001"}))
	if err == nil || err.Error() != "already_shipped" {
		t.Fatalf("dup ship err=%v", err)
	}
	if got := diamondsOf(t, s, "10001"); got != before+6 {
		t.Fatalf("dup ship credited diamonds: %d", got)
	}
}

func TestBannedClaimRejectedViaHandlers(t *testing.T) {
	s := testStore(t)
	raw, err := s.mailSend(contextTODO(), mustJSON(t, map[string]any{
		"playerId": "10007", "title": "误发", "attachments": []any{map[string]any{"itemId": "item_gem", "count": 1}},
	}))
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	mailID := mapResult(t, raw)["mailId"].(string)
	_, err = s.mailClaim(contextTODO(), mustJSON(t, map[string]any{"mailId": mailID}))
	if err == nil || err.Error() != "player_banned" {
		t.Fatalf("claim err=%v want player_banned", err)
	}
}

func TestReportGenerateViaHandler(t *testing.T) {
	s := testStore(t)
	raw, err := s.reportGenerate(contextTODO(), mustJSON(t, map[string]any{"range": "daily", "delayMs": 0}))
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	res := mapResult(t, raw)
	if res["success"] != true || res["runSeq"].(float64) != 1 {
		t.Fatalf("report result=%v", res)
	}
	if !strings.HasPrefix(res["reportId"].(string), "report-rpg-") {
		t.Fatalf("reportId=%v", res["reportId"])
	}

	// Cancelled context: no run recorded, runSeq unchanged.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.reportGenerate(ctx, mustJSON(t, map[string]any{"delayMs": 0})); err == nil {
		t.Fatal("cancelled report must fail")
	}
	if _, err := s.reportGenerate(contextTODO(), mustJSON(t, map[string]any{"delayMs": 0})); err != nil {
		t.Fatalf("report after cancel: %v", err)
	}
	s.mu.RLock()
	seq := s.reportRunSeq
	s.mu.RUnlock()
	if seq != 2 {
		t.Fatalf("runSeq=%d want 2 (cancel never records)", seq)
	}
}

func TestTaskListAndRefreshViaHandlers(t *testing.T) {
	s := testStore(t)
	raw, err := s.taskList(contextTODO(), mustJSON(t, map[string]any{"playerId": "10001", "pageSize": 50}))
	if err != nil {
		t.Fatalf("taskList: %v", err)
	}
	items := mapResult(t, raw)["items"].([]any)
	if len(items) != 6 {
		t.Fatalf("quests=%d want 6", len(items))
	}
	// daily filter
	raw, _ = s.taskList(contextTODO(), mustJSON(t, map[string]any{"daily": true, "pageSize": 50}))
	if got := len(mapResult(t, raw)["items"].([]any)); got != 3 {
		t.Fatalf("daily quests=%d want 3", got)
	}
	// refresh: stale nothing today — refreshed must be 0, success true
	raw, err = s.taskRefresh(contextTODO(), mustJSON(t, map[string]any{}))
	if err != nil {
		t.Fatalf("taskRefresh: %v", err)
	}
	if mapResult(t, raw)["refreshed"].(float64) != 0 {
		t.Fatalf("same-day refresh=%s", raw)
	}
}

func TestTaskClaimViaHandler(t *testing.T) {
	s := testStore(t)
	raw, err := s.taskClaim(contextTODO(), mustJSON(t, map[string]any{"playerId": "10001", "questId": "q_login_daily"}))
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	res := mapResult(t, raw)
	if res["success"] != true || res["gold"].(float64) != 200 {
		t.Fatalf("claim result=%v", res)
	}
}

func TestEconomySnapshotAndAuditViaHandlers(t *testing.T) {
	s := testStore(t)
	if _, err := s.economySnapshot(contextTODO(), mustJSON(t, map[string]any{})); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	raw, err := s.economyAudit(contextTODO(), mustJSON(t, map[string]any{"action": "player.ban", "pageSize": 10}))
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	res := mapResult(t, raw)
	if res["total"].(float64) != 1 {
		t.Fatalf("ban audit total=%v", res["total"])
	}
}
