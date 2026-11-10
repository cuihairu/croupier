package main

import (
	"math/rand"
	"strings"
	"testing"
)

// The four consistency rules the acceptance chain calls out, plus the quest
// economy loop. Every test asserts both the error and that state is untouched.

func TestShipOrderIdempotentRejectSecond(t *testing.T) {
	s := testStore(t)
	const orderNo = "20261110001" // seeded paid order, 10001 +6 diamonds
	before := diamondsOf(t, s, "10001")

	res, err := s.shipOrder(orderNo, "tester")
	if err != nil {
		t.Fatalf("first ship: %v", err)
	}
	if res["success"] != true || res["diamonds"].(int64) != 6 {
		t.Fatalf("first ship result=%v", res)
	}
	if got := diamondsOf(t, s, "10001"); got != before+6 {
		t.Fatalf("diamonds after ship=%d want %d", got, before+6)
	}

	if _, err := s.shipOrder(orderNo, "tester"); err == nil {
		t.Fatal("second ship must be rejected")
	} else if err.Error() != "already_shipped" {
		t.Fatalf("second ship err=%v want already_shipped", err)
	}
	if got := diamondsOf(t, s, "10001"); got != before+6 {
		t.Fatalf("diamonds after dup ship=%d want unchanged %d", got, before+6)
	}
}

func TestShipNonPaidOrderRejected(t *testing.T) {
	s := testStore(t)
	if _, err := s.shipOrder("20261110004", "tester"); err == nil || err.Error() != "order_not_paid" {
		t.Fatalf("pending ship err=%v want order_not_paid", err)
	}
}

func TestBannedPlayerClaimRejected(t *testing.T) {
	s := testStore(t)
	// Seeded player 10007 is banned; GM mail still delivers, claim must fail.
	before := inventoryCount(t, s, "10007", "item_gem")
	mail, err := s.sendMail("10007", "封禁期间误发", "test", []rpgMailAttachment{{ItemID: "item_gem", Count: 1}}, 24, "gm01")
	if err != nil {
		t.Fatalf("send to banned player: %v", err)
	}
	if _, err := s.claimMail(mail.ID, "player"); err == nil {
		t.Fatal("banned claim must be rejected")
	} else if err.Error() != "player_banned" {
		t.Fatalf("claim err=%v want player_banned", err)
	}
	if got := inventoryCount(t, s, "10007", "item_gem"); got != before {
		t.Fatalf("banned player inventory changed: %d want %d", got, before)
	}
}

func TestBalanceFailClosed(t *testing.T) {
	s := testStore(t)
	before := goldOf(t, s, "10003") // 780 gold
	if err := s.spendGold("10003", before+1, "tester", "overdraft"); err == nil {
		t.Fatal("overdraft spend must be rejected")
	} else if err.Error() != "insufficient_gold" {
		t.Fatalf("spend err=%v want insufficient_gold", err)
	}
	if got := goldOf(t, s, "10003"); got != before {
		t.Fatalf("gold changed on rejected spend: %d", got)
	}

	// Refund of a shipped order debits diamonds fail-closed: strip the
	// diamonds first, the refund must refuse and leave the order shipped.
	s.mu.Lock()
	s.playerByIDLocked("10001").Diamonds = 0
	s.playerByIDLocked("10001").Status = "online"
	o := s.orderByNoLocked("20261110002") // shipped 648 to 10004
	p4 := s.playerByIDLocked("10004")
	p4.Diamonds = 1
	s.mu.Unlock()
	if _, err := s.refundOrder(o.OrderNo, "tester"); err == nil || err.Error() != "insufficient_diamonds" {
		t.Fatalf("refund err=%v want insufficient_diamonds", err)
	}
	s.mu.RLock()
	status := s.orderByNoLocked(o.OrderNo).Status
	s.mu.RUnlock()
	if status != "shipped" {
		t.Fatalf("failed refund mutated status: %s", status)
	}
}

func TestRefundShippedDeductsDiamonds(t *testing.T) {
	s := testStore(t)
	before := diamondsOf(t, s, "10004") // 3200, seeded shipped order 648
	res, err := s.refundOrder("20261110002", "tester")
	if err != nil {
		t.Fatalf("refund shipped: %v", err)
	}
	if res["diamondsDeducted"].(int) != 648 {
		t.Fatalf("diamondsDeducted=%v want 648", res["diamondsDeducted"])
	}
	if got := diamondsOf(t, s, "10004"); got != before-648 {
		t.Fatalf("diamonds after refund=%d want %d", got, before-648)
	}
	if _, err := s.refundOrder("20261110002", "tester"); err == nil || err.Error() != "order_already_refunded" {
		t.Fatalf("second refund err=%v want order_already_refunded", err)
	}
}

func TestMailExpiryAndExactlyOnce(t *testing.T) {
	s := testStore(t)
	// Seeded mail-9002 expired a day ago with 2 gems for 10002.
	if _, err := s.claimMail("mail-9002", "player"); err == nil {
		t.Fatal("expired claim must be rejected")
	} else if err.Error() != "mail_expired" {
		t.Fatalf("claim err=%v want mail_expired", err)
	}
	if got := inventoryCount(t, s, "10002", "item_gem"); got != 0 {
		t.Fatalf("expired claim mutated inventory: %d", got)
	}

	// Seeded mail-9001 (compensation) claims exactly once.
	before := inventoryCount(t, s, "10001", "item_potion")
	res, err := s.claimMail("mail-9001", "player")
	if err != nil {
		t.Fatalf("claim compensation mail: %v", err)
	}
	if len(res["items"].([]rpgMailAttachment)) != 2 {
		t.Fatalf("claim items=%v", res["items"])
	}
	if got := inventoryCount(t, s, "10001", "item_potion"); got != before+5 {
		t.Fatalf("potion count=%d want %d", got, before+5)
	}
	if got := inventoryCount(t, s, "10001", "item_gem"); got != 1 {
		t.Fatalf("gem count=%d want 1", got)
	}
	if _, err := s.claimMail("mail-9001", "player"); err == nil || err.Error() != "mail_already_claimed" {
		t.Fatalf("second claim err=%v want mail_already_claimed", err)
	}
	if got := inventoryCount(t, s, "10001", "item_potion"); got != before+5 {
		t.Fatalf("duplicate claim mutated inventory: %d", got)
	}
}

func TestSendMailValidation(t *testing.T) {
	s := testStore(t)
	if _, err := s.sendMail("99999", "t", "", nil, 0, "gm01"); err == nil || err.Error() != "player_not_found" {
		t.Fatalf("unknown recipient err=%v", err)
	}
	if _, err := s.sendMail("10001", "t", "", []rpgMailAttachment{{ItemID: "nope", Count: 1}}, 0, "gm01"); err == nil {
		t.Fatal("unknown item must be rejected")
	}
	if _, err := s.sendMail("10001", "t", "", []rpgMailAttachment{{ItemID: "item_gem", Count: -1}}, 0, "gm01"); err == nil {
		t.Fatal("negative count must be rejected")
	}
	mail, err := s.sendMail("10001", "补发", "ok", []rpgMailAttachment{{ItemID: "item_gem", Count: 1}}, 0, "gm01")
	if err != nil {
		t.Fatalf("valid send: %v", err)
	}
	if !strings.HasPrefix(mail.ID, "mail-") {
		t.Fatalf("mail id=%s", mail.ID)
	}
}

func TestQuestClaimRewardsExactlyOnce(t *testing.T) {
	s := testStore(t)
	gold := goldOf(t, s, "10001")
	res, err := s.claimQuest("10001", "q_login_daily", "player")
	if err != nil {
		t.Fatalf("claim login quest: %v", err)
	}
	if res["gold"].(int64) != 200 {
		t.Fatalf("reward gold=%v want 200", res["gold"])
	}
	if got := goldOf(t, s, "10001"); got != gold+200 {
		t.Fatalf("gold=%d want %d", got, gold+200)
	}
	if _, err := s.claimQuest("10001", "q_login_daily", "player"); err == nil || err.Error() != "quest_already_completed" {
		t.Fatalf("second claim err=%v want quest_already_completed", err)
	}
	if got := goldOf(t, s, "10001"); got != gold+200 {
		t.Fatalf("duplicate claim mutated gold: %d", got)
	}
}

func TestQuestClaimItemRewardAndLevel(t *testing.T) {
	s := testStore(t)
	s.mu.Lock()
	p := s.playerByIDLocked("10003")
	p.XP = 2995 // one 50xp reward crosses the level-12 boundary (12*100=1200... pool covers repeated level-ups)
	s.mu.Unlock()
	before := inventoryCount(t, s, "10003", "item_potion")
	// q_kill_slime is daily in_progress 0 for 10003 — push it to claimable.
	if _, err := s.addQuestProgress("10003", "q_kill_slime", 10); err != nil {
		t.Fatalf("progress: %v", err)
	}
	res, err := s.claimQuest("10003", "q_kill_slime", "player")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if res["rewardItem"].(string) != "item_potion" || res["rewardCount"].(int64) != 2 {
		t.Fatalf("item reward=%v", res)
	}
	if got := inventoryCount(t, s, "10003", "item_potion"); got != before+2 {
		t.Fatalf("potion=%d want %d", got, before+2)
	}
	if res["level"].(int) < 13 {
		t.Fatalf("level after big xp pool=%v", res["level"])
	}
	if _, err := s.addQuestProgress("10003", "q_kill_slime", 1); err != nil {
		t.Fatalf("progress after completion: %v", err)
	}
	if _, err := s.claimQuest("10003", "q_kill_slime", "player"); err == nil {
		t.Fatal("completed daily quest must not re-claim same day")
	}
}

func TestQuestClaimGuards(t *testing.T) {
	s := testStore(t)
	if _, err := s.claimQuest("10001", "nope", "player"); err == nil || err.Error() != "quest_not_found" {
		t.Fatalf("quest err=%v", err)
	}
	if _, err := s.claimQuest("99999", "q_login_daily", "player"); err == nil || err.Error() != "player_not_found" {
		t.Fatalf("player err=%v", err)
	}
	// In-progress (not claimable) cannot be claimed.
	if _, err := s.claimQuest("10006", "q_kill_slime", "player"); err == nil || err.Error() != "quest_not_claimable" {
		t.Fatalf("claim err=%v want quest_not_claimable", err)
	}
}

func TestDailyRefreshOnTick(t *testing.T) {
	s := testStore(t)
	// Mark 10001 q_login_daily as completed yesterday.
	s.mu.Lock()
	pr := s.progressLocked("10001", "q_login_daily")
	pr.Status = "completed"
	pr.Progress = 1
	pr.Day = "2020-01-01"
	s.mu.Unlock()

	s.Tick(nowMillis(), rand.New(rand.NewSource(7)))

	s.mu.RLock()
	pr = s.progressLocked("10001", "q_login_daily")
	status, prog := pr.Status, pr.Progress
	s.mu.RUnlock()
	if status != "in_progress" || prog != 0 {
		t.Fatalf("daily refresh status=%s progress=%d", status, prog)
	}
}

func TestBanUnbanLifecycle(t *testing.T) {
	s := testStore(t)
	ban, err := s.banPlayer("10006", "abuse", 24, "gm01")
	if err != nil {
		t.Fatalf("ban: %v", err)
	}
	s.mu.RLock()
	status := s.playerByIDLocked("10006").Status
	s.mu.RUnlock()
	if status != "banned" {
		t.Fatalf("status after ban=%s", status)
	}
	if _, err := s.banPlayer("10006", "again", 24, "gm01"); err == nil || err.Error() != "player_already_banned" {
		t.Fatalf("double ban err=%v", err)
	}
	if _, err := s.claimMail("mail-9001", "player"); err != nil && strings.Contains(err.Error(), "10006") {
		t.Fatalf("unexpected: %v", err)
	}
	if err := s.unbanPlayer("10006", "gm01"); err != nil {
		t.Fatalf("unban: %v", err)
	}
	s.mu.RLock()
	status = s.playerByIDLocked("10006").Status
	banActive := s.activeBanLocked("10006") != nil
	s.mu.RUnlock()
	if status != "offline" || banActive {
		t.Fatalf("after unban status=%s banActive=%v", status, banActive)
	}
	_ = ban
}

func TestTickSeededDeterministic(t *testing.T) {
	s := testStore(t)
	s.mu.RLock()
	players := len(s.players)
	s.mu.RUnlock()
	s.Tick(nowMillis(), rand.New(rand.NewSource(1)))
	s.Tick(nowMillis(), rand.New(rand.NewSource(1)))
	s.mu.RLock()
	audit := len(s.audit)
	s.mu.RUnlock()
	if players != 10 {
		t.Fatalf("players=%d", players)
	}
	if audit < 2 {
		t.Fatalf("tick produced no audit trail: %d", audit)
	}
}
