// Package main implements the gameops-rpg demo server ("复杂游戏 demo").
//
// Unlike gameops-demo (pure CRUD mirror of a browser seed), this demo models
// an RPG with intrinsic state — players with currencies and inventories,
// items, quests with progress, mail with claimable attachments (the
// compensation flow), orders with idempotent shipping, and bans linked to
// player status. A periodic tick evolves the world so data stays alive.
// All callable function contracts read and write this one shared store, and
// the whole world persists to a JSON snapshot (statePath) across restarts.
package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
)

// ---------------------------------------------------------------------------
// Record types (JSON field names are lowerCamelCase contract surface)
// ---------------------------------------------------------------------------

type rpgItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // resource | equipment | consumable
	Tradable bool   `json:"tradable"`
}

type rpgPlayer struct {
	ID         string           `json:"id"`
	Nickname   string           `json:"nickname"`
	Level      int              `json:"level"`
	XP         int64            `json:"xp"`
	Gold       int64            `json:"gold"`
	Diamonds   int64            `json:"diamonds"`
	Inventory  map[string]int64 `json:"inventory"` // itemId -> count
	Status     string           `json:"status"`    // online | offline | banned
	VIP        int              `json:"vip"`
	LastSeenAt string           `json:"lastSeenAt"`
	CreatedAt  string           `json:"createdAt"`
}

type rpgProduct struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Currency string `json:"currency"`
	Diamonds int64  `json:"diamonds"`
}

type rpgQuestTemplate struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	GoalType        string `json:"goalType"` // kill | spend | recharge | login | collect | level
	GoalCount       int64  `json:"goalCount"`
	RewardGold      int64  `json:"rewardGold"`
	RewardXP        int64  `json:"rewardXp"`
	RewardItemID    string `json:"rewardItemId,omitempty"`
	RewardItemCount int64  `json:"rewardItemCount,omitempty"`
	Daily           bool   `json:"daily"`
}

type rpgQuestProgress struct {
	PlayerID    string  `json:"playerId"`
	QuestID     string  `json:"questId"`
	Progress    int64   `json:"progress"`
	Status      string  `json:"status"` // in_progress | claimable | completed
	Day         string  `json:"day"`    // UTC calendar day scoping daily quests
	CompletedAt *string `json:"completedAt,omitempty"`
}

type rpgMailAttachment struct {
	ItemID string `json:"itemId"`
	Count  int64  `json:"count"`
}

type rpgMail struct {
	ID          string              `json:"id"`
	PlayerID    string              `json:"playerId"`
	Title       string              `json:"title"`
	Content     string              `json:"content"`
	Attachments []rpgMailAttachment `json:"attachments"`
	ExpireAt    string              `json:"expireAt"`
	Read        bool                `json:"read"`
	Claimed     bool                `json:"claimed"`
	ClaimedAt   *string             `json:"claimedAt,omitempty"`
	Sender      string              `json:"sender"`
	CreatedAt   string              `json:"createdAt"`
}

type rpgOrder struct {
	OrderNo     string  `json:"orderNo"`
	PlayerID    string  `json:"playerId"`
	ProductID   string  `json:"productId"`
	ProductName string  `json:"productName"`
	Amount      int64   `json:"amount"`
	Currency    string  `json:"currency"`
	Diamonds    int64   `json:"diamonds"`
	Status      string  `json:"status"` // pending | paid | shipped | refunded
	ShipKey     string  `json:"shipKey"`
	ShippedAt   *string `json:"shippedAt,omitempty"`
	CreatedAt   string  `json:"createdAt"`
}

type rpgBan struct {
	ID       string `json:"id"`
	PlayerID string `json:"playerId"`
	Reason   string `json:"reason"`
	StartAt  string `json:"startAt"`
	EndAt    string `json:"endAt"`
	Operator string `json:"operator"`
	Active   bool   `json:"active"`
}

type rpgAuditRecord struct {
	ID     string `json:"id"`
	At     string `json:"at"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Target string `json:"target"`
	Result string `json:"result"`
	Detail string `json:"detail,omitempty"`
}

// snapshotVersion guards forward compatibility of the persisted world file.
const snapshotVersion = 1

// auditMax bounds the in-memory audit trail; oldest records drop off.
const auditMax = 500

type rpgSnapshot struct {
	Version      int                `json:"version"`
	Players      []rpgPlayer        `json:"players"`
	Items        []rpgItem          `json:"items"`
	Products     []rpgProduct       `json:"products"`
	Quests       []rpgQuestTemplate `json:"quests"`
	Progress     []rpgQuestProgress `json:"progress"`
	Mails        []rpgMail          `json:"mails"`
	Orders       []rpgOrder         `json:"orders"`
	Bans         []rpgBan           `json:"bans"`
	Audit        []rpgAuditRecord   `json:"audit"`
	ReportRunSeq int                `json:"reportRunSeq"`
	MailSeq      int                `json:"mailSeq"`
	OrderSeq     int                `json:"orderSeq"`
	BanSeq       int                `json:"banSeq"`
	AuditSeq     int                `json:"auditSeq"`
}

// rpgStore is the mutable shared world and the only state surface the
// executors mutate. A single instance is shared across all registered
// functions and the tick loop; rand is only touched while mu is held.
type rpgStore struct {
	mu        sync.RWMutex
	rand      *rand.Rand
	statePath string

	players  []rpgPlayer
	items    []rpgItem
	products []rpgProduct
	quests   []rpgQuestTemplate
	progress []rpgQuestProgress
	mails    []rpgMail
	orders   []rpgOrder
	bans     []rpgBan
	audit    []rpgAuditRecord

	reportRunSeq int
	mailSeq      int
	orderSeq     int
	banSeq       int
	auditSeq     int
}

func strPtr(v string) *string { return &v }

// newRPGStore builds the deterministic seed; when statePath is set the saved
// snapshot (if any) replaces the seed so the world survives restarts.
func newRPGStore(rng *rand.Rand, statePath ...string) *rpgStore {
	s := &rpgStore{rand: rng}
	s.seedLocked()
	if len(statePath) > 0 && statePath[0] != "" {
		s.statePath = statePath[0]
		s.loadStateLocked()
	}
	return s
}

// loadStateLocked replaces the seed with the persisted snapshot when the file
// exists and parses. It runs before any concurrent access (construction
// time), so no lock is taken for the load itself.
func (s *rpgStore) loadStateLocked() {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		return // first run or unreadable state: start from the seed
	}
	var snap rpgSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		log.Printf("gameops-rpg: state file unreadable, starting from seed: %v", err)
		return
	}
	if snap.Version != snapshotVersion {
		log.Printf("gameops-rpg: state version %d unsupported (want %d), starting from seed", snap.Version, snapshotVersion)
		return
	}
	s.players = snap.Players
	s.items = snap.Items
	s.products = snap.Products
	s.quests = snap.Quests
	s.progress = snap.Progress
	s.mails = snap.Mails
	s.orders = snap.Orders
	s.bans = snap.Bans
	s.audit = snap.Audit
	s.reportRunSeq = snap.ReportRunSeq
	s.mailSeq = snap.MailSeq
	s.orderSeq = snap.OrderSeq
	s.banSeq = snap.BanSeq
	s.auditSeq = snap.AuditSeq
	if s.items == nil {
		s.seedItemsLocked()
	}
	if s.products == nil {
		s.seedProductsLocked()
	}
	if s.quests == nil {
		s.seedQuestsLocked()
	}
}

// persistStateLocked writes the full world snapshot to statePath atomically
// (temp file + rename). Callers must hold s.mu. A write failure is logged and
// non-fatal: the demo keeps running in memory.
func (s *rpgStore) persistStateLocked() {
	if s.statePath == "" {
		return
	}
	snap := rpgSnapshot{
		Version:      snapshotVersion,
		Players:      s.players,
		Items:        s.items,
		Products:     s.products,
		Quests:       s.quests,
		Progress:     s.progress,
		Mails:        s.mails,
		Orders:       s.orders,
		Bans:         s.bans,
		Audit:        s.audit,
		ReportRunSeq: s.reportRunSeq,
		MailSeq:      s.mailSeq,
		OrderSeq:     s.orderSeq,
		BanSeq:       s.banSeq,
		AuditSeq:     s.auditSeq,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		log.Printf("gameops-rpg: marshal state failed: %v", err)
		return
	}
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("gameops-rpg: persist state failed: %v", err)
		return
	}
	if err := os.Rename(tmp, s.statePath); err != nil {
		// Rename can fail across filesystems; fall back to a direct write.
		if err := os.WriteFile(s.statePath, data, 0o600); err != nil {
			log.Printf("gameops-rpg: persist state failed: %v", err)
			return
		}
		_ = os.Remove(tmp)
	}
	if dir := filepath.Dir(s.statePath); dir != "." {
		_ = dir // filepath kept for future dir-creation handling
	}
}

// ---------------------------------------------------------------------------
// Deterministic seed
// ---------------------------------------------------------------------------

func (s *rpgStore) seedLocked() {
	now := nowMillis()
	iso := func(msAgo int64) string { return millisISO(now - msAgo) }
	future := func(msAhead int64) string { return millisISO(now + msAhead) }
	today := dayKey(now)

	s.seedItemsLocked()
	s.seedProductsLocked()
	s.seedQuestsLocked()

	s.players = []rpgPlayer{
		{ID: "10001", Nickname: "剑舞流萤", Level: 24, XP: 640, Gold: 5200, Diamonds: 180,
			Inventory: map[string]int64{"item_potion": 8, "item_wood": 14, "item_sword": 1},
			Status:    "online", VIP: 3, LastSeenAt: iso(0), CreatedAt: iso(120 * 86_400_000)},
		{ID: "10002", Nickname: "北岭雪狼", Level: 31, XP: 210, Gold: 9100, Diamonds: 60,
			Inventory: map[string]int64{"item_stone": 30, "item_shield": 1},
			Status:    "offline", VIP: 1, LastSeenAt: iso(5 * 3_600_000), CreatedAt: iso(200 * 86_400_000)},
		{ID: "10003", Nickname: "月下独酌", Level: 12, XP: 300, Gold: 780, Diamonds: 0,
			Inventory: map[string]int64{"item_potion": 2},
			Status:    "online", VIP: 0, LastSeenAt: iso(0), CreatedAt: iso(30 * 86_400_000)},
		{ID: "10004", Nickname: "千机阁主", Level: 45, XP: 900, Gold: 24000, Diamonds: 3200,
			Inventory: map[string]int64{"item_gem": 6, "item_sword": 2, "item_ticket": 4},
			Status:    "online", VIP: 6, LastSeenAt: iso(0), CreatedAt: iso(365 * 86_400_000)},
		{ID: "10005", Nickname: "阿柴", Level: 5, XP: 90, Gold: 120, Diamonds: 0,
			Inventory: map[string]int64{"item_wood": 3},
			Status:    "offline", VIP: 0, LastSeenAt: iso(30 * 3_600_000), CreatedAt: iso(2 * 86_400_000)},
		{ID: "10006", Nickname: "风陵渡口", Level: 19, XP: 450, Gold: 3300, Diamonds: 36,
			Inventory: map[string]int64{"item_stone": 12, "item_potion": 5},
			Status:    "online", VIP: 2, LastSeenAt: iso(0), CreatedAt: iso(60 * 86_400_000)},
		{ID: "10007", Nickname: "暗夜刀锋", Level: 28, XP: 500, Gold: 15000, Diamonds: 880,
			Inventory: map[string]int64{"item_gem": 2},
			Status:    "banned", VIP: 4, LastSeenAt: iso(26 * 3_600_000), CreatedAt: iso(90 * 86_400_000)},
		{ID: "10008", Nickname: "奶妈不奶", Level: 22, XP: 380, Gold: 2600, Diamonds: 12,
			Inventory: map[string]int64{"item_potion": 15, "item_mana_scroll": 6},
			Status:    "online", VIP: 1, LastSeenAt: iso(0), CreatedAt: iso(75 * 86_400_000)},
		{ID: "10009", Nickname: "钓佬永不空军", Level: 8, XP: 140, Gold: 540, Diamonds: 6,
			Inventory: map[string]int64{"item_wood": 9, "item_stone": 4},
			Status:    "offline", VIP: 0, LastSeenAt: iso(12 * 3_600_000), CreatedAt: iso(10 * 86_400_000)},
		{ID: "10010", Nickname: "一刀九百九", Level: 37, XP: 1200, Gold: 12800, Diamonds: 1500,
			Inventory: map[string]int64{"item_ticket": 2, "item_sword": 1},
			Status:    "online", VIP: 5, LastSeenAt: iso(0), CreatedAt: iso(150 * 86_400_000)},
	}

	s.bans = []rpgBan{{
		ID: "ban-3001", PlayerID: "10007", Reason: "cheating",
		StartAt: iso(26 * 3_600_000), EndAt: future(46 * 3_600_000),
		Operator: "gm01", Active: true,
	}}

	s.mails = []rpgMail{
		{
			ID: "mail-9001", PlayerID: "10001", Title: "10-21 回档补偿",
			Content:     "感谢理解，补偿见附件。",
			Attachments: []rpgMailAttachment{{ItemID: "item_potion", Count: 5}, {ItemID: "item_gem", Count: 1}},
			ExpireAt:    future(48 * 3_600_000), Sender: "gm01", CreatedAt: iso(2 * 3_600_000),
		},
		{
			ID: "mail-9002", PlayerID: "10002", Title: "赛季结算奖励",
			Content:     "S3 赛季结算，奖励见附件。",
			Attachments: []rpgMailAttachment{{ItemID: "item_gem", Count: 2}},
			ExpireAt:    iso(24 * 3_600_000), Sender: "system", CreatedAt: iso(8 * 24 * 3_600_000),
		},
		{
			ID: "mail-9003", PlayerID: "10003", Title: "新手礼包",
			Content:     "欢迎来到艾泽拉……欢迎来到新世界。",
			Attachments: []rpgMailAttachment{{ItemID: "item_potion", Count: 3}},
			ExpireAt:    future(72 * 3_600_000), Read: true, Claimed: true,
			ClaimedAt: strPtr(iso(20 * 3_600_000)), Sender: "system", CreatedAt: iso(24 * 3_600_000),
		},
	}

	s.orders = []rpgOrder{
		{OrderNo: "20261110001", PlayerID: "10001", ProductID: "prod_small", ProductName: "6 钻石礼包",
			Amount: 6, Currency: "CNY", Diamonds: 6, Status: "paid", ShipKey: "ship-20261110001",
			CreatedAt: iso(1 * 3_600_000)},
		{OrderNo: "20261110002", PlayerID: "10004", ProductID: "prod_mega", ProductName: "648 钻石礼包",
			Amount: 648, Currency: "CNY", Diamonds: 648, Status: "shipped", ShipKey: "ship-20261110002",
			ShippedAt: strPtr(iso(20 * 3_600_000)), CreatedAt: iso(22 * 3_600_000)},
		{OrderNo: "20261110003", PlayerID: "10003", ProductID: "prod_small", ProductName: "6 钻石礼包",
			Amount: 6, Currency: "CNY", Diamonds: 6, Status: "refunded", ShipKey: "ship-20261110003",
			CreatedAt: iso(40 * 3_600_000)},
		{OrderNo: "20261110004", PlayerID: "10006", ProductID: "prod_med", ProductName: "30 钻石礼包",
			Amount: 30, Currency: "CNY", Diamonds: 30, Status: "pending", ShipKey: "ship-20261110004",
			CreatedAt: iso(30 * 60_000)},
	}

	s.progress = []rpgQuestProgress{
		{PlayerID: "10001", QuestID: "q_login_daily", Progress: 1, Status: "claimable", Day: today},
		{PlayerID: "10001", QuestID: "q_kill_slime", Progress: 7, Status: "in_progress", Day: today},
		{PlayerID: "10002", QuestID: "q_collect_wood", Progress: 20, Status: "completed",
			Day: today, CompletedAt: strPtr(iso(6 * 3_600_000))},
		{PlayerID: "10003", QuestID: "q_login_daily", Progress: 1, Status: "completed",
			Day: today, CompletedAt: strPtr(iso(4 * 3_600_000))},
		{PlayerID: "10004", QuestID: "q_recharge_once", Progress: 1, Status: "claimable", Day: today},
		{PlayerID: "10006", QuestID: "q_kill_slime", Progress: 2, Status: "in_progress", Day: today},
		{PlayerID: "10008", QuestID: "q_spend_gold", Progress: 300, Status: "in_progress", Day: today},
	}

	s.audit = []rpgAuditRecord{
		{ID: "audit-7001", At: iso(2 * 3_600_000), Actor: "gm01", Action: "mail.send",
			Target: "mail-9001", Result: "success", Detail: "回档补偿附件 item_potion x5, item_gem x1"},
		{ID: "audit-7002", At: iso(26 * 3_600_000), Actor: "gm01", Action: "player.ban",
			Target: "10007", Result: "success", Detail: "cheating / 72h"},
	}
	s.mailSeq, s.orderSeq, s.banSeq, s.auditSeq = 9003, 20261110004, 3001, 7002
	s.reportRunSeq = 0
}

func (s *rpgStore) seedItemsLocked() {
	s.items = []rpgItem{
		{ID: "item_potion", Name: "生命药水", Type: "consumable", Tradable: true},
		{ID: "item_mana_scroll", Name: "法力卷轴", Type: "consumable", Tradable: true},
		{ID: "item_ticket", Name: "竞技场门票", Type: "consumable", Tradable: false},
		{ID: "item_wood", Name: "硬木", Type: "resource", Tradable: true},
		{ID: "item_stone", Name: "花岗岩", Type: "resource", Tradable: true},
		{ID: "item_gem", Name: "魔力宝石", Type: "resource", Tradable: true},
		{ID: "item_sword", Name: "新手长剑", Type: "equipment", Tradable: true},
		{ID: "item_shield", Name: "青铜盾", Type: "equipment", Tradable: false},
	}
}

func (s *rpgStore) seedProductsLocked() {
	s.products = []rpgProduct{
		{ID: "prod_small", Name: "6 钻石礼包", Price: 6, Currency: "CNY", Diamonds: 6},
		{ID: "prod_med", Name: "30 钻石礼包", Price: 30, Currency: "CNY", Diamonds: 30},
		{ID: "prod_big", Name: "98 钻石礼包", Price: 98, Currency: "CNY", Diamonds: 98},
		{ID: "prod_huge", Name: "198 钻石礼包", Price: 198, Currency: "CNY", Diamonds: 198},
		{ID: "prod_mega", Name: "648 钻石礼包", Price: 648, Currency: "CNY", Diamonds: 648},
	}
}

func (s *rpgStore) seedQuestsLocked() {
	s.quests = []rpgQuestTemplate{
		{ID: "q_login_daily", Title: "每日签到", GoalType: "login", GoalCount: 1,
			RewardGold: 200, RewardXP: 50, Daily: true},
		{ID: "q_kill_slime", Title: "讨伐史莱姆", GoalType: "kill", GoalCount: 10,
			RewardGold: 500, RewardXP: 100, RewardItemID: "item_potion", RewardItemCount: 2, Daily: true},
		{ID: "q_spend_gold", Title: "挥金如土", GoalType: "spend", GoalCount: 1000,
			RewardGold: 300, RewardXP: 80, Daily: true},
		{ID: "q_recharge_once", Title: "首充有礼", GoalType: "recharge", GoalCount: 1,
			RewardItemID: "item_ticket", RewardItemCount: 1},
		{ID: "q_collect_wood", Title: "伐木累", GoalType: "collect", GoalCount: 20,
			RewardGold: 800, RewardXP: 150, RewardItemID: "item_wood", RewardItemCount: 5},
		{ID: "q_reach_level", Title: "百炼成钢", GoalType: "level", GoalCount: 20,
			RewardItemID: "item_gem", RewardItemCount: 3},
	}
}

// ---------------------------------------------------------------------------
// Lookups and shared mutations (callers hold the appropriate lock)
// ---------------------------------------------------------------------------

func (s *rpgStore) playerByIDLocked(id string) *rpgPlayer {
	for i := range s.players {
		if s.players[i].ID == id {
			return &s.players[i]
		}
	}
	return nil
}

func (s *rpgStore) itemByIDLocked(id string) *rpgItem {
	for i := range s.items {
		if s.items[i].ID == id {
			return &s.items[i]
		}
	}
	return nil
}

func (s *rpgStore) questByIDLocked(id string) *rpgQuestTemplate {
	for i := range s.quests {
		if s.quests[i].ID == id {
			return &s.quests[i]
		}
	}
	return nil
}

func (s *rpgStore) orderByNoLocked(no string) *rpgOrder {
	for i := range s.orders {
		if s.orders[i].OrderNo == no {
			return &s.orders[i]
		}
	}
	return nil
}

func (s *rpgStore) mailByIDLocked(id string) *rpgMail {
	for i := range s.mails {
		if s.mails[i].ID == id {
			return &s.mails[i]
		}
	}
	return nil
}

func (s *rpgStore) progressLocked(playerID, questID string) *rpgQuestProgress {
	for i := range s.progress {
		if s.progress[i].PlayerID == playerID && s.progress[i].QuestID == questID {
			return &s.progress[i]
		}
	}
	return nil
}

func (s *rpgStore) activeBanLocked(playerID string) *rpgBan {
	for i := range s.bans {
		if s.bans[i].PlayerID == playerID && s.bans[i].Active {
			return &s.bans[i]
		}
	}
	return nil
}

func (s *rpgStore) productByIDLocked(id string) *rpgProduct {
	for i := range s.products {
		if s.products[i].ID == id {
			return &s.products[i]
		}
	}
	return nil
}

// pushAuditLocked appends an audit record and caps the trail. Callers hold
// s.mu (write).
func (s *rpgStore) pushAuditLocked(actor, action, target, result, detail string) string {
	s.auditSeq++
	id := "audit-" + itoa(s.auditSeq)
	s.audit = append([]rpgAuditRecord{{
		ID: id, At: millisISO(nowMillis()), Actor: actor, Action: action,
		Target: target, Result: result, Detail: detail,
	}}, s.audit...)
	if len(s.audit) > auditMax {
		s.audit = s.audit[:auditMax]
	}
	return id
}

// nextMailIDLocked/nextOrderNoLocked/nextBanIDLocked allocate sequence-based IDs.
func (s *rpgStore) nextMailIDLocked() string {
	s.mailSeq++
	return "mail-" + itoa(s.mailSeq)
}

func (s *rpgStore) nextOrderNoLocked() string {
	s.orderSeq++
	return itoa(s.orderSeq)
}

func (s *rpgStore) nextBanIDLocked() string {
	s.banSeq++
	return "ban-" + itoa(s.banSeq)
}

func nowMillis() int64 { return millisNow() }
