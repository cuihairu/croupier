package main

import (
	"fmt"
	"math/rand"
)

// World behaviours: the intrinsic gameplay loop plus the consistency rules
// the acceptance chain exercises (idempotent shipping, banned claim
// rejection, fail-closed balances, mail expiry). All mutating methods take
// and release s.mu themselves so executors and the tick loop share one code
// path.

// ---------------------------------------------------------------------------
// Balance rules (fail-closed: rejected mutations never touch state)
// ---------------------------------------------------------------------------

// spendGold debits gold; insufficient balance is rejected fail-closed and
// the player is untouched.
func (s *rpgStore) spendGold(playerID string, amount int64, actor, reason string) error {
	if amount <= 0 {
		return fmt.Errorf("invalid_amount")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.playerByIDLocked(playerID)
	if p == nil {
		return fmt.Errorf("player_not_found")
	}
	if p.Gold < amount {
		return fmt.Errorf("insufficient_gold")
	}
	p.Gold -= amount
	s.pushAuditLocked(actor, "economy.spend", playerID, "success",
		fmt.Sprintf("-%d gold (%s), balance %d", amount, reason, p.Gold))
	s.persistStateLocked()
	return nil
}

// grantCurrencies credits gold/diamonds (never negative amounts).
func (s *rpgStore) grantCurrencies(playerID string, gold, diamonds int64, actor, reason string) error {
	if gold < 0 || diamonds < 0 {
		return fmt.Errorf("invalid_amount")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.playerByIDLocked(playerID)
	if p == nil {
		return fmt.Errorf("player_not_found")
	}
	p.Gold += gold
	p.Diamonds += diamonds
	s.pushAuditLocked(actor, "economy.grant", playerID, "success",
		fmt.Sprintf("+%d gold +%d diamonds (%s)", gold, diamonds, reason))
	s.persistStateLocked()
	return nil
}

// grantItems credits inventory items; unknown items and non-positive counts
// are rejected.
func (s *rpgStore) grantItems(playerID string, items []rpgMailAttachment, actor, reason string) error {
	if len(items) == 0 {
		return fmt.Errorf("no_items")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.playerByIDLocked(playerID)
	if p == nil {
		return fmt.Errorf("player_not_found")
	}
	for _, att := range items {
		if att.Count <= 0 {
			return fmt.Errorf("invalid_item_count")
		}
		if s.itemByIDLocked(att.ItemID) == nil {
			return fmt.Errorf("item_not_found: %s", att.ItemID)
		}
	}
	if p.Inventory == nil {
		p.Inventory = map[string]int64{}
	}
	for _, att := range items {
		p.Inventory[att.ItemID] += att.Count
	}
	s.pushAuditLocked(actor, "economy.grant_items", playerID, "success", reason)
	s.persistStateLocked()
	return nil
}

// addQuestProgress advances one quest goal by amount and flips the progress
// to claimable when the goal is reached. It returns the updated progress.
func (s *rpgStore) addQuestProgress(playerID, questID string, amount int64) (*rpgQuestProgress, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("invalid_amount")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.playerByIDLocked(playerID) == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	q := s.questByIDLocked(questID)
	if q == nil {
		return nil, fmt.Errorf("quest_not_found")
	}
	if b := s.activeBanLocked(playerID); b != nil {
		return nil, fmt.Errorf("player_banned")
	}
	pr := s.ensureProgressLocked(playerID, q)
	if pr.Status == "completed" {
		return pr, nil
	}
	pr.Progress += amount
	if pr.Progress >= q.GoalCount {
		pr.Progress = q.GoalCount
		pr.Status = "claimable"
	}
	s.persistStateLocked()
	return pr, nil
}

// ensureProgressLocked returns the day-scoped progress row for a daily quest
// (refreshing it when the stored day is stale) or a fresh row for one-shot
// quests. Callers hold s.mu (write).
func (s *rpgStore) ensureProgressLocked(playerID string, q *rpgQuestTemplate) *rpgQuestProgress {
	today := dayKey(nowMillis())
	if pr := s.progressLocked(playerID, q.ID); pr != nil {
		if q.Daily && pr.Day != today {
			pr.Progress = 0
			pr.Status = "in_progress"
			pr.Day = today
			pr.CompletedAt = nil
		}
		return pr
	}
	row := &rpgQuestProgress{PlayerID: playerID, QuestID: q.ID, Status: "in_progress", Day: today}
	s.progress = append(s.progress, *row)
	return s.progressLocked(playerID, q.ID)
}

// claimQuest grants the rewards of a claimable quest. Daily quests complete
// for their day; one-shot quests complete forever.
func (s *rpgStore) claimQuest(playerID, questID, actor string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.playerByIDLocked(playerID)
	if p == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	if b := s.activeBanLocked(playerID); b != nil {
		return nil, fmt.Errorf("player_banned")
	}
	q := s.questByIDLocked(questID)
	if q == nil {
		return nil, fmt.Errorf("quest_not_found")
	}
	pr := s.ensureProgressLocked(playerID, q)
	if pr.Status == "completed" {
		return nil, fmt.Errorf("quest_already_completed")
	}
	if pr.Status != "claimable" {
		return nil, fmt.Errorf("quest_not_claimable")
	}
	now := millisISO(nowMillis())
	pr.Status = "completed"
	pr.CompletedAt = &now
	p.Gold += q.RewardGold
	p.XP += q.RewardXP
	s.levelUpLocked(p)
	if q.RewardItemID != "" && q.RewardItemCount > 0 {
		if p.Inventory == nil {
			p.Inventory = map[string]int64{}
		}
		p.Inventory[q.RewardItemID] += q.RewardItemCount
	}
	s.pushAuditLocked(actor, "task.claim", playerID+"/"+questID, "success",
		fmt.Sprintf("gold+%d xp+%d item %s x%d", q.RewardGold, q.RewardXP, q.RewardItemID, q.RewardItemCount))
	s.persistStateLocked()
	return map[string]any{
		"success":     true,
		"playerId":    playerID,
		"questId":     questID,
		"gold":        q.RewardGold,
		"xp":          q.RewardXP,
		"rewardItem":  q.RewardItemID,
		"rewardCount": q.RewardItemCount,
		"level":       p.Level,
	}, nil
}

// levelUpLocked applies XP thresholds: level N costs N*100 XP, levelling
// repeatedly while the pool covers the next cost.
func (s *rpgStore) levelUpLocked(p *rpgPlayer) {
	for p.XP >= int64(p.Level)*100 {
		p.XP -= int64(p.Level) * 100
		p.Level++
	}
}

// refreshDailyLocked resets stale daily progress rows. Callers hold s.mu.
func (s *rpgStore) refreshDailyLocked() {
	today := dayKey(nowMillis())
	for i := range s.progress {
		pr := &s.progress[i]
		q := s.questByIDLocked(pr.QuestID)
		if q == nil || !q.Daily || pr.Day == today {
			continue
		}
		pr.Progress = 0
		pr.Status = "in_progress"
		pr.Day = today
		pr.CompletedAt = nil
	}
}

// ---------------------------------------------------------------------------
// Mail: the compensation flow core
// ---------------------------------------------------------------------------

// sendMail delivers mail with attachments to one player. Attachments are
// validated against the item catalog up front so compensation never queues a
// claim that would fail.
func (s *rpgStore) sendMail(playerID, title, content string, attachments []rpgMailAttachment, expireHours int64, actor string) (*rpgMail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.playerByIDLocked(playerID)
	if p == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	if title == "" {
		return nil, fmt.Errorf("title_required")
	}
	for _, att := range attachments {
		if att.Count <= 0 {
			return nil, fmt.Errorf("invalid_item_count")
		}
		if s.itemByIDLocked(att.ItemID) == nil {
			return nil, fmt.Errorf("item_not_found: %s", att.ItemID)
		}
	}
	if expireHours <= 0 {
		expireHours = 72
	}
	now := nowMillis()
	s.mailSeq++
	mail := rpgMail{
		ID:          "mail-" + itoa(s.mailSeq),
		PlayerID:    playerID,
		Title:       title,
		Content:     content,
		Attachments: attachments,
		ExpireAt:    millisISO(now + expireHours*3_600_000),
		Sender:      actor,
		CreatedAt:   millisISO(now),
	}
	s.mails = append([]rpgMail{mail}, s.mails...)
	s.pushAuditLocked(actor, "mail.send", mail.ID, "success",
		fmt.Sprintf("to %s, %d attachment(s)", playerID, len(attachments)))
	s.persistStateLocked()
	return &mail, nil
}

// claimMail moves mail attachments into the player inventory. Rejections:
// unknown mail, already claimed, expired mail, banned recipient. A successful
// claim is exactly-once: the mail flips to claimed under the store lock.
func (s *rpgStore) claimMail(mailID, actor string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.mailByIDLocked(mailID)
	if m == nil {
		return nil, fmt.Errorf("mail_not_found")
	}
	if m.Claimed {
		return nil, fmt.Errorf("mail_already_claimed")
	}
	now := nowMillis()
	expire, err := timeParseRFC3339(m.ExpireAt)
	if err != nil {
		return nil, fmt.Errorf("invalid_expire_at: %w", err)
	}
	if expire <= now {
		return nil, fmt.Errorf("mail_expired")
	}
	p := s.playerByIDLocked(m.PlayerID)
	if p == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	if b := s.activeBanLocked(m.PlayerID); b != nil {
		return nil, fmt.Errorf("player_banned")
	}
	if len(m.Attachments) == 0 {
		m.Claimed = true
		m.Read = true
		claimedAt := millisISO(now)
		m.ClaimedAt = &claimedAt
		s.persistStateLocked()
		return map[string]any{"success": true, "mailId": mailID, "items": []rpgMailAttachment{}}, nil
	}
	if p.Inventory == nil {
		p.Inventory = map[string]int64{}
	}
	for _, att := range m.Attachments {
		p.Inventory[att.ItemID] += att.Count
	}
	m.Claimed = true
	m.Read = true
	claimedAt := millisISO(now)
	m.ClaimedAt = &claimedAt
	s.pushAuditLocked(actor, "mail.claim", mailID, "success",
		fmt.Sprintf("%s claimed %d attachment(s)", m.PlayerID, len(m.Attachments)))
	s.persistStateLocked()
	return map[string]any{"success": true, "mailId": mailID, "items": m.Attachments}, nil
}

// mailExpiredLocked reports whether the mail is past its expiry. Callers hold
// at least a read lock.
func (s *rpgStore) mailExpiredLocked(m *rpgMail) bool {
	expire, err := timeParseRFC3339(m.ExpireAt)
	if err != nil {
		return false
	}
	return expire <= nowMillis()
}

// ---------------------------------------------------------------------------
// Orders: idempotent shipping
// ---------------------------------------------------------------------------

// shipOrder grants the order's diamonds exactly once. A second ship call on
// the same order is rejected (already_shipped) — the observable idempotency
// evidence — and non-paid orders cannot ship at all.
func (s *rpgStore) shipOrder(orderNo, actor string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.orderByNoLocked(orderNo)
	if o == nil {
		return nil, fmt.Errorf("order_not_found")
	}
	if o.Status == "shipped" {
		return nil, fmt.Errorf("already_shipped")
	}
	if o.Status != "paid" {
		return nil, fmt.Errorf("order_not_paid")
	}
	p := s.playerByIDLocked(o.PlayerID)
	if p == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	now := millisISO(nowMillis())
	o.Status = "shipped"
	o.ShippedAt = &now
	p.Diamonds += o.Diamonds
	s.pushAuditLocked(actor, "order.ship", orderNo, "success",
		fmt.Sprintf("%s +%d diamonds", o.PlayerID, o.Diamonds))
	s.persistStateLocked()
	return map[string]any{"success": true, "orderNo": orderNo, "playerId": o.PlayerID, "diamonds": o.Diamonds}, nil
}

// refundOrder cancels an order. Shipping diamonds are debited back
// fail-closed: a player who already spent them cannot go negative, and the
// order stays untouched when the debit is refused.
func (s *rpgStore) refundOrder(orderNo, actor string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.orderByNoLocked(orderNo)
	if o == nil {
		return nil, fmt.Errorf("order_not_found")
	}
	if o.Status == "refunded" {
		return nil, fmt.Errorf("order_already_refunded")
	}
	p := s.playerByIDLocked(o.PlayerID)
	if p == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	wasShipped := o.Status == "shipped"
	if wasShipped {
		if p.Diamonds < o.Diamonds {
			return nil, fmt.Errorf("insufficient_diamonds")
		}
		p.Diamonds -= o.Diamonds
	}
	o.Status = "refunded"
	s.pushAuditLocked(actor, "order.refund", orderNo, "success",
		fmt.Sprintf("%s refunded %d %s", o.PlayerID, o.Amount, o.Currency))
	s.persistStateLocked()
	return map[string]any{
		"success":          true,
		"orderNo":          orderNo,
		"playerId":         o.PlayerID,
		"diamondsDeducted": ternaryInt(wasShipped, int(o.Diamonds), 0),
	}, nil
}

// ---------------------------------------------------------------------------
// Bans
// ---------------------------------------------------------------------------

// banPlayer creates the ban record and flips the player status to banned
// (online players are kicked offline by the ban).
func (s *rpgStore) banPlayer(playerID, reason string, durationHours int64, actor string) (*rpgBan, error) {
	if durationHours <= 0 {
		return nil, fmt.Errorf("duration_must_be_positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.playerByIDLocked(playerID)
	if p == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	if b := s.activeBanLocked(playerID); b != nil {
		return nil, fmt.Errorf("player_already_banned")
	}
	now := nowMillis()
	s.banSeq++
	ban := rpgBan{
		ID: "ban-" + itoa(s.banSeq), PlayerID: playerID, Reason: reason,
		StartAt: millisISO(now), EndAt: millisISO(now + durationHours*3_600_000),
		Operator: actor, Active: true,
	}
	s.bans = append([]rpgBan{ban}, s.bans...)
	p.Status = "banned"
	s.pushAuditLocked(actor, "player.ban", playerID, "success",
		fmt.Sprintf("%s / %dh", reason, durationHours))
	s.persistStateLocked()
	return &ban, nil
}

// unbanPlayer lifts every active ban and restores the player to offline.
func (s *rpgStore) unbanPlayer(playerID, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.playerByIDLocked(playerID)
	if p == nil {
		return fmt.Errorf("player_not_found")
	}
	lifted := 0
	for i := range s.bans {
		if s.bans[i].PlayerID == playerID && s.bans[i].Active {
			s.bans[i].Active = false
			lifted++
		}
	}
	if lifted == 0 {
		return fmt.Errorf("player_not_banned")
	}
	if p.Status == "banned" {
		p.Status = "offline"
	}
	s.pushAuditLocked(actor, "player.unban", playerID, "success", fmt.Sprintf("%d ban(s) lifted", lifted))
	s.persistStateLocked()
	return nil
}

// expireBansLocked deactivates bans past their end time and restores the
// player status. Callers hold s.mu (write).
func (s *rpgStore) expireBansLocked() {
	now := nowMillis()
	for i := range s.bans {
		b := &s.bans[i]
		if !b.Active {
			continue
		}
		end, err := timeParseRFC3339(b.EndAt)
		if err != nil || end > now {
			continue
		}
		b.Active = false
		if p := s.playerByIDLocked(b.PlayerID); p != nil && p.Status == "banned" {
			p.Status = "offline"
		}
	}
}

// ---------------------------------------------------------------------------
// Tick: the living world
// ---------------------------------------------------------------------------

// Tick advances the simulation one step: ban expiry, daily quest refresh,
// then random behaviours for online players (quest progress, consumption,
// recharge creating paid orders). rng is injectable so tests are
// deterministic; every mutation is audited and persisted like a handler call.
func (s *rpgStore) Tick(now int64, rng *rand.Rand) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rand = rng
	s.expireBansLocked()
	s.refreshDailyLocked()
	for i := range s.players {
		p := &s.players[i]
		switch p.Status {
		case "banned":
			continue
		case "offline":
			if rng.Intn(100) < 15 {
				p.Status = "online"
				p.LastSeenAt = millisISO(now)
			}
			continue
		case "online":
			if rng.Intn(100) < 8 {
				p.Status = "offline"
				p.LastSeenAt = millisISO(now)
				continue
			}
			s.behaveLocked(p, now, rng)
		}
	}
	s.persistStateLocked()
}

// behaveLocked picks one behaviour for an online player. Callers hold s.mu.
func (s *rpgStore) behaveLocked(p *rpgPlayer, now int64, rng *rand.Rand) {
	roll := rng.Intn(100)
	switch {
	case roll < 35: // quest progress on the first non-completed quest
		if pr := s.firstOpenQuestLocked(p.ID); pr != nil {
			q := s.questByIDLocked(pr.QuestID)
			if q != nil {
				pr.Progress += 1
				if pr.Progress >= q.GoalCount {
					pr.Progress = q.GoalCount
					pr.Status = "claimable"
				}
			}
		}
	case roll < 60: // consume a stackable resource item if any
		s.consumeItemLocked(p, rng)
	case roll < 85: // recharge: a paid order awaiting operator ship
		s.createRechargeOrderLocked(p, rng, now)
	default: // idle touch
		p.LastSeenAt = millisISO(now)
	}
}

// firstOpenQuestLocked returns the player's first quest that can still take
// progress (in_progress on today's day key). Callers hold s.mu.
func (s *rpgStore) firstOpenQuestLocked(playerID string) *rpgQuestProgress {
	today := dayKey(nowMillis())
	for i := range s.progress {
		pr := &s.progress[i]
		if pr.PlayerID != playerID || pr.Status != "in_progress" {
			continue
		}
		if q := s.questByIDLocked(pr.QuestID); q != nil && q.Daily && pr.Day != today {
			continue
		}
		return pr
	}
	return nil
}

// consumeItemLocked spends gold on a consumable from the inventory
// (economy loop: rewards → inventory → consumption). Callers hold s.mu.
func (s *rpgStore) consumeItemLocked(p *rpgPlayer, rng *rand.Rand) {
	const potionCost = int64(50)
	if p.Gold < potionCost {
		return // fail-closed: nothing happens on insufficient balance
	}
	if p.Inventory["item_potion"] <= 0 {
		return
	}
	p.Gold -= potionCost
	p.Inventory["item_potion"]--
	s.pushAuditLocked("system", "economy.consume", p.ID, "success",
		fmt.Sprintf("used item_potion, -%d gold, balance %d", potionCost, p.Gold))
}

// createRechargeOrderLocked creates a paid order for a random product; it
// stays pending-ship until an operator calls order.ship. Callers hold s.mu.
func (s *rpgStore) createRechargeOrderLocked(p *rpgPlayer, rng *rand.Rand, now int64) {
	if len(s.products) == 0 {
		return
	}
	prod := s.products[rng.Intn(len(s.products))]
	s.orderSeq++
	order := rpgOrder{
		OrderNo: itoa(s.orderSeq), PlayerID: p.ID,
		ProductID: prod.ID, ProductName: prod.Name,
		Amount: prod.Price, Currency: prod.Currency, Diamonds: prod.Diamonds,
		Status: "paid", ShipKey: "ship-" + itoa(s.orderSeq),
		CreatedAt: millisISO(now),
	}
	s.orders = append([]rpgOrder{order}, s.orders...)
	s.pushAuditLocked("system", "order.create", order.OrderNo, "success",
		fmt.Sprintf("%s bought %s (paid, awaiting ship)", p.ID, prod.ID))
}
