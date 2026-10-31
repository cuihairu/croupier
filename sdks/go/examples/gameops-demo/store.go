// Package main implements the game-ops demo server ("demo-server 基础").
//
// It mirrors the E1 browser seed (web/src/components/PageRenderer/templates/
// gameOpsDemoData.ts) as a process-local, mutable store so every callable
// function contract reads and writes the same shared sample state. Reset
// restores the deterministic initial seed; operations are visible to
// subsequent reads (write-then-read), which is the E2 requirement.
package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// ---------------------------------------------------------------------------
// Record types (JSON field names must stay aligned with the E1 web seed)
// ---------------------------------------------------------------------------

type gameOpsServer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type gameOpsPlayer struct {
	ID          string `json:"id"`
	Nickname    string `json:"nickname"`
	Character   string `json:"character"`
	ServerID    string `json:"serverId"`
	Level       int    `json:"level"`
	VIP         int    `json:"vip"`
	Status      string `json:"status"`
	BalanceGold int64  `json:"balanceGold"`
	LastLoginAt string `json:"lastLoginAt"`
	CreatedAt   string `json:"createdAt"`
}

type gameOpsTask struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"`
	Title           string   `json:"title"`
	Detail          string   `json:"detail"`
	ReceiverRange   string   `json:"receiverRange"`
	TargetServerID  *string  `json:"targetServerId"`
	TargetPlayerID  *string  `json:"targetPlayerId"`
	TargetPlayerIDs []string `json:"targetPlayerIds"`
	TargetOrderNo   *string  `json:"targetOrderNo"`
	TargetCount     int      `json:"targetCount"`
	PerAmount       *int64   `json:"perAmount"`
	TotalAmount     *int64   `json:"totalAmount"`
	Currency        *string  `json:"currency"`
	Status          string   `json:"status"`
	SentCount       int      `json:"sentCount"`
	FailedCount     int      `json:"failedCount"`
	Operator        string   `json:"operator"`
	ApprovalID      *string  `json:"approvalId"`
	CreatedAt       string   `json:"createdAt"`
}

type gameOpsBan struct {
	ID            string `json:"id"`
	PlayerID      string `json:"playerId"`
	Nickname      string `json:"nickname"`
	ServerID      string `json:"serverId"`
	ApprovalID    string `json:"approvalId"`
	Reason        string `json:"reason"`
	DurationHours int    `json:"durationHours"`
	BannedAt      string `json:"bannedAt"`
	ExpiresAt     string `json:"expiresAt"`
	Status        string `json:"status"`
	Operator      string `json:"operator"`
	Approver      string `json:"approver"`
}

type gameOpsOrder struct {
	OrderNo        string  `json:"orderNo"`
	PlayerID       string  `json:"playerId"`
	Nickname       string  `json:"nickname"`
	Amount         int64   `json:"amount"`
	Currency       string  `json:"currency"`
	Channel        string  `json:"channel"`
	Status         string  `json:"status"`
	GameCredit     string  `json:"gameCredit"`
	Reconciliation string  `json:"reconciliation"`
	PayTransaction string  `json:"payTransactionId"`
	GameCreditRef  *string `json:"gameCreditRef"`
	PaidAt         string  `json:"paidAt"`
	CreditedAt     *string `json:"creditedAt"`
}

type gameOpsActivityVersion struct {
	Version     int    `json:"version"`
	Note        string `json:"note"`
	PublishedAt string `json:"publishedAt"`
	Operator    string `json:"operator"`
}

type gameOpsActivity struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	Type           string                   `json:"type"`
	Timezone       string                   `json:"timezone"`
	StartAt        string                   `json:"startAt"`
	EndAt          string                   `json:"endAt"`
	Status         string                   `json:"status"`
	CurrentVersion int                      `json:"currentVersion"`
	Versions       []gameOpsActivityVersion `json:"versions"`
}

type gameOpsOpRecord struct {
	ID          string  `json:"id"`
	At          string  `json:"at"`
	Operator    string  `json:"operator"`
	Action      string  `json:"action"`
	Target      string  `json:"target"`
	Result      string  `json:"result"`
	OperationID string  `json:"operationId"`
	ApprovalID  *string `json:"approvalId"`
}

// gameOpsStore is the mutable shared demo dataset and the only state surface
// the executors mutate. A single instance is shared across all registered
// functions; Reset returns it to the deterministic initial seed.
type gameOpsStore struct {
	mu         sync.RWMutex
	servers    []gameOpsServer
	players    []gameOpsPlayer
	tasks      []gameOpsTask
	bans       []gameOpsBan
	orders     []gameOpsOrder
	activities []gameOpsActivity
	opRecords  []gameOpsOpRecord
	opSeq      int
	// reportRunSeq counts actual report.generate executions. The platform's
	// idempotency-key dedupe happens before the handler runs, so a retried
	// invocation with the same key leaves this counter unchanged — the
	// observable evidence for the E2 idempotent-retry scenario.
	reportRunSeq int
	// statePath, when non-empty, is a JSON file the demo uses to persist
	// reportRunSeq across process restarts. The platform idempotency dedupe
	// runs before the handler, but the handler's own run counter must survive
	// a restart so a re-delivered report.generate does not re-emit a
	// reportId that was already handed out (UI-002 process-restart evidence).
	statePath string
}

var longNickname = "风起云涌一区无敌战神丶樱花漫天飞舞的超级无敌可爱小龙女本尊在此"

func strPtr(v string) *string { return &v }
func int64Ptr(v int64) *int64 { return &v }

// newGameOpsStore builds a fresh deterministic seed (matches E1 browser seed).
func newGameOpsStore(statePath ...string) *gameOpsStore {
	s := &gameOpsStore{}
	s.resetLocked()
	s.opSeq = len(s.opRecords)
	if len(statePath) > 0 && statePath[0] != "" {
		s.statePath = statePath[0]
		s.loadStateLocked()
	}
	return s
}

// loadStateLocked restores reportRunSeq from statePath if the file exists.
// It runs before any concurrent access (construction time), so no lock is
// taken for the load itself.
func (s *gameOpsStore) loadStateLocked() {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		return // first run or unreadable state: start from the seed
	}
	var st struct {
		ReportRunSeq int `json:"reportRunSeq"`
	}
	if json.Unmarshal(data, &st) == nil && st.ReportRunSeq > s.reportRunSeq {
		s.reportRunSeq = st.ReportRunSeq
	}
}

// persistStateLocked writes the current reportRunSeq to statePath. Callers
// must hold s.mu. A write failure is non-fatal: the demo keeps running and
// the counter stays in memory (a restart after a failed persist may re-hand
// out a reportId, which is the documented best-effort demo behavior).
func (s *gameOpsStore) persistStateLocked() {
	if s.statePath == "" {
		return
	}
	st := struct {
		ReportRunSeq int `json:"reportRunSeq"`
	}{ReportRunSeq: s.reportRunSeq}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	if err := os.WriteFile(s.statePath, data, 0o600); err != nil {
		log.Printf("persist report state failed: %v", err)
	}
}

func (s *gameOpsStore) resetLocked() {
	now := nowMillis()
	iso := func(msAgo int64) string { return millisISO(now - msAgo) }

	s.servers = []gameOpsServer{
		{ID: "s1", Name: "电信一区", Status: "healthy"},
		{ID: "s2", Name: "电信二区", Status: "busy"},
		{ID: "s3", Name: "联通一区", Status: "maintenance"},
	}

	s.players = make([]gameOpsPlayer, 0, 26)
	for i := 0; i < 26; i++ {
		seq := i + 1
		name := "玩家" + intStr(seq) + "号"
		if seq == 13 {
			name = longNickname
		}
		s.players = append(s.players, gameOpsPlayer{
			ID:          "200" + padLeft(intStr(seq), 2, '0'),
			Nickname:    name,
			Character:   "角色" + intStr(seq),
			ServerID:    s.servers[seq%len(s.servers)].ID,
			Level:       5 + ((seq * 7) % 55),
			VIP:         seq % 7,
			Status:      ternary(seq%7 == 0, "banned", "active"),
			BalanceGold: int64((seq * 977) % 9000),
			LastLoginAt: iso(int64(seq) * 3_600_000),
			CreatedAt:   iso(int64(seq) * 86_400_000 * 3),
		})
	}

	s.tasks = []gameOpsTask{
		{
			ID: "task-mail-1001", Type: "mail", Title: "S3 赛季结算奖励",
			Detail:        "恭喜完成 S3 赛季，奖励已随信发放。",
			ReceiverRange: "all", TargetPlayerIDs: []string{}, TargetCount: 26,
			Status: "sent", SentCount: 26, Operator: "gm01",
			CreatedAt: iso(49 * 3_600_000),
		},
		{
			ID: "task-mail-1002", Type: "mail", Title: "停机维护补偿",
			Detail:        "感谢理解，维护补偿见附件。",
			ReceiverRange: "server", TargetServerID: strPtr("s1"), TargetPlayerIDs: []string{}, TargetCount: 9,
			Status: "partial_failed", SentCount: 7, FailedCount: 2, Operator: "operator",
			CreatedAt: iso(20 * 3_600_000),
		},
		{
			ID: "task-comp-2001", Type: "compensation", Title: "回档补偿",
			Detail:        "10-21 回档事故全员补偿",
			ReceiverRange: "all", TargetPlayerIDs: []string{}, TargetCount: 26,
			PerAmount: int64Ptr(500), TotalAmount: int64Ptr(13000), Currency: strPtr("金"),
			Status: "pending_approval", Operator: "operator", ApprovalID: strPtr("approval-comp-2001"),
			CreatedAt: iso(2 * 3_600_000),
		},
		{
			ID: "task-comp-2002", Type: "compensation", Title: "充值未到账补偿",
			Detail:        "订单 20261023001 补发",
			ReceiverRange: "players", TargetPlayerID: strPtr("20008"), TargetPlayerIDs: []string{"20008"}, TargetOrderNo: strPtr("2026102308"),
			TargetCount: 1, PerAmount: int64Ptr(648), TotalAmount: int64Ptr(648), Currency: strPtr("金"),
			Status: "completed", SentCount: 1, Operator: "gm01", ApprovalID: strPtr("approval-comp-2002"),
			CreatedAt: iso(30 * 3_600_000),
		},
	}

	reasons := []string{"cheating", "fraud", "abuse", "risk"}
	s.bans = make([]gameOpsBan, 0, 12)
	for i := 0; i < 12; i++ {
		seq := i + 1
		player := s.players[(seq*3)%len(s.players)]
		duration := []int{24, 72, 168, 720}[seq%4]
		status := "active"
		if seq%3 == 0 {
			status = "expired"
		} else if seq%5 == 0 {
			status = "lifted"
		}
		s.bans = append(s.bans, gameOpsBan{
			ID:       "ban-300" + padLeft(intStr(seq), 2, '0'),
			PlayerID: player.ID, Nickname: player.Nickname, ServerID: player.ServerID,
			ApprovalID: "approval-ban-" + padLeft(intStr(seq), 4, '0'),
			Reason:     reasons[seq%len(reasons)], DurationHours: duration,
			BannedAt:  iso(int64(seq+2) * 86_400_000),
			ExpiresAt: iso(int64(seq+2)*86_400_000 - int64(duration)*3_600_000),
			Status:    status,
			Operator:  ternary(seq%2 == 0, "operator", "gm01"), Approver: "admin",
		})
	}

	channels := []string{"official", "ios", "android", "wechat", "alipay"}
	s.orders = make([]gameOpsOrder, 0, 30)
	for i := 0; i < 30; i++ {
		seq := i + 1
		player := s.players[(seq*5)%len(s.players)]
		missingCredit := seq == 8 || seq == 17
		missingOrder := seq == 24
		failed := seq%11 == 0
		gameCredit := "credited"
		if failed {
			gameCredit = "not_needed"
		} else if missingCredit {
			gameCredit = "missing"
		} else if seq == 26 {
			gameCredit = "pending"
		}
		status := "credited"
		if failed {
			status = "failed"
		} else if missingCredit || seq == 26 {
			status = "paid"
		}
		reconciliation := "match"
		if missingCredit {
			reconciliation = "missing_credit"
		} else if missingOrder {
			reconciliation = "missing_order"
		}
		var ref *string
		var creditedAt *string
		if gameCredit == "credited" {
			ref = strPtr("credit-" + intStr(7000+seq))
			creditedAt = strPtr(iso(int64(seq)*5*3_600_000 - 60_000))
		}
		s.orders = append(s.orders, gameOpsOrder{
			OrderNo: "2026102" + intStr(300+seq), PlayerID: player.ID, Nickname: player.Nickname,
			Amount:   int64([]int{6, 30, 98, 328, 648}[seq%5]),
			Currency: ternary(seq%9 == 0, "USD", "CNY"), Channel: channels[seq%len(channels)],
			Status: status, GameCredit: gameCredit, Reconciliation: reconciliation,
			PayTransaction: "pay-" + intStr(9000+seq), GameCreditRef: ref,
			PaidAt: iso(int64(seq) * 5 * 3_600_000), CreditedAt: creditedAt,
		})
	}

	activityTypes := []string{"login_gift", "rank_reward", "shop_discount", "holiday_event"}
	timezones := []string{"Asia/Shanghai", "UTC", "America/Los_Angeles"}
	s.activities = make([]gameOpsActivity, 0, 8)
	for i := 0; i < 8; i++ {
		seq := i + 1
		versionCount := (seq % 3) + 1
		versions := make([]gameOpsActivityVersion, 0, versionCount)
		for v := 0; v < versionCount; v++ {
			note := "初版配置"
			if v > 0 {
				note = "调整奖励数值（v" + intStr(v+1) + "）"
			}
			versions = append(versions, gameOpsActivityVersion{
				Version: v + 1, Note: note,
				PublishedAt: iso(int64(versionCount-v) * 86_400_000),
				Operator:    ternary(v%2 == 0, "gm01", "operator"),
			})
		}
		status := "ended"
		switch seq % 4 {
		case 0:
			status = "active"
		case 1:
			status = "configured"
		case 2:
			status = "published"
		}
		s.activities = append(s.activities, gameOpsActivity{
			ID:   "act-400" + intStr(seq),
			Name: []string{"每日登录礼", "巅峰排行榜", "限时折扣商店", "节日庆典"}[seq%4] + "·第" + intStr(seq) + "期",
			Type: activityTypes[seq%len(activityTypes)], Timezone: timezones[seq%len(timezones)],
			StartAt: iso(-int64(seq) * 7 * 86_400_000), EndAt: iso(int64(seq) * 86_400_000),
			Status: status, CurrentVersion: versionCount, Versions: versions,
		})
	}

	s.opRecords = []gameOpsOpRecord{
		{ID: "op-5001", At: iso(26 * 3_600_000), Operator: "operator", Action: "player.ban",
			Target: "ban-30003", Result: "success", OperationID: "op-5001", ApprovalID: strPtr("approval-ban-0003")},
		{ID: "op-5002", At: iso(2 * 3_600_000), Operator: "operator", Action: "compensation.grant",
			Target: "task-comp-2001", Result: "pending_approval", OperationID: "op-5002", ApprovalID: strPtr("approval-comp-2001")},
		{ID: "op-5003", At: iso(8 * 3_600_000), Operator: "gm01", Action: "activity.publish",
			Target: "act-4003", Result: "success", OperationID: "op-5003"},
	}
	s.opSeq = len(s.opRecords)
	s.reportRunSeq = 0
}

// Reset restores the initial deterministic seed.
func (s *gameOpsStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetLocked()
}

// PushOpRecord prepends an operation record and returns its operationId.
func (s *gameOpsStore) pushOpLocked(operator, action, target, result string, approvalID *string) string {
	s.opSeq++
	id := "op-demo-" + intStr(s.opSeq)
	s.opRecords = append([]gameOpsOpRecord{{
		ID: id, At: millisISO(nowMillis()), Operator: operator, Action: action,
		Target: target, Result: result, OperationID: id, ApprovalID: approvalID,
	}}, s.opRecords...)
	return id
}

func nowMillis() int64 {
	return millisNow()
}

func intStr(v int) string {
	return itoa(v)
}

func padLeft(s string, n int, pad byte) string {
	for len(s) < n {
		s = string(pad) + s
	}
	return s
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
