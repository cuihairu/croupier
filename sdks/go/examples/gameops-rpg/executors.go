package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Executors bind the world behaviours to function contracts. Thin layer:
// decode payload → call one store/world method → encode result.

type pageParams struct {
	current  int
	pageSize int
	value    map[string]any
}

func newPageParams(m map[string]any) pageParams {
	return pageParams{current: valueInt(m, "current"), pageSize: valueInt(m, "pageSize"), value: m}
}

func (p pageParams) normalized() (int, int) {
	cur := p.current
	if cur < 1 {
		cur = 1
	}
	size := p.pageSize
	if size < 1 {
		size = 10
	}
	return cur, size
}

func (p pageParams) start() int {
	cur, size := p.normalized()
	return (cur - 1) * size
}

func (p pageParams) size() int {
	_, size := p.normalized()
	return size
}

func slicePage[T any](items []T, p pageParams) []T {
	start := p.start()
	if start >= len(items) {
		return []T{}
	}
	end := start + p.size()
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func collection(items any, total int, p pageParams) map[string]any {
	cur, size := p.normalized()
	return map[string]any{"items": items, "total": total, "page": cur, "pageSize": size}
}

// --- player.* ----------------------------------------------------------------

func (s *rpgStore) playerList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]rpgPlayer, 0, len(s.players))
	for _, p := range s.players {
		if q := valueString(m, "nickname"); q != "" && !strings.Contains(p.Nickname, q) {
			continue
		}
		if q := valueString(m, "status"); q != "" && p.Status != q {
			continue
		}
		filtered = append(filtered, p)
	}
	sortSliceBy(m, filtered)
	return encodeJSON(collection(slicePage(filtered, pp), len(filtered), pp))
}

func (s *rpgStore) playerGet(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.playerByIDLocked(valueString(m, "id"))
	if p == nil {
		return []byte("null"), nil
	}
	return encodeJSON(p)
}

func (s *rpgStore) playerInventory(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.playerByIDLocked(valueString(m, "playerId"))
	if p == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	type itemEntry struct {
		ItemID string `json:"itemId"`
		Name   string `json:"name"`
		Type   string `json:"type"`
		Count  int64  `json:"count"`
	}
	items := make([]itemEntry, 0, len(p.Inventory))
	total := int64(0)
	for id, count := range p.Inventory {
		entry := itemEntry{ItemID: id, Count: count}
		if it := s.itemByIDLocked(id); it != nil {
			entry.Name = it.Name
			entry.Type = it.Type
		}
		items = append(items, entry)
		total += count
	}
	return encodeJSON(map[string]any{
		"playerId": p.ID, "nickname": p.Nickname, "items": items, "totalItems": total,
	})
}

func (s *rpgStore) playerBan(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	ban, err := s.banPlayer(valueString(m, "playerId"),
		strOr(valueString(m, "reason"), "risk"),
		int64Or(valueInt(m, "durationHours"), 72), "platform")
	if err != nil {
		return nil, err
	}
	return encodeJSON(map[string]any{"success": true, "id": ban.ID, "playerId": ban.PlayerID})
}

func (s *rpgStore) playerUnban(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	if err := s.unbanPlayer(valueString(m, "playerId"), "platform"); err != nil {
		return nil, err
	}
	return encodeJSON(map[string]any{"success": true, "playerId": valueString(m, "playerId")})
}

// --- mail.* ------------------------------------------------------------------

func (s *rpgStore) mailSend(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	mail, err := s.sendMail(valueString(m, "playerId"), valueString(m, "title"),
		valueString(m, "content"), valueAttachmentSlice(m, "attachments"),
		valueInt64(m, "expireHours"), "platform")
	if err != nil {
		return nil, err
	}
	return encodeJSON(map[string]any{"success": true, "mailId": mail.ID, "playerId": mail.PlayerID})
}

func (s *rpgStore) mailList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	type mailView struct {
		rpgMail
		Expired bool `json:"expired"`
	}
	filtered := make([]mailView, 0, len(s.mails))
	for _, mail := range s.mails {
		if q := valueString(m, "playerId"); q != "" && mail.PlayerID != q {
			continue
		}
		view := mailView{rpgMail: mail, Expired: s.mailExpiredLocked(&mail)}
		filtered = append(filtered, view)
	}
	// mails are stored newest-first; no orderBy re-sort (embedded view type is
	// not reflect-sortable).
	return encodeJSON(collection(slicePage(filtered, pp), len(filtered), pp))
}

func (s *rpgStore) mailClaim(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	result, err := s.claimMail(valueString(m, "mailId"), "player")
	if err != nil {
		return nil, err
	}
	return encodeJSON(result)
}

// --- task.* (quests) ----------------------------------------------------------

func (s *rpgStore) taskList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := valueString(m, "playerId")
	dailyOnly := valueBool(m, "daily")
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	type questView struct {
		rpgQuestTemplate
		Progress *rpgQuestProgress `json:"progress"`
	}
	filtered := make([]questView, 0, len(s.quests))
	for _, q := range s.quests {
		if dailyOnly && !q.Daily {
			continue
		}
		view := questView{rpgQuestTemplate: q}
		if playerID != "" {
			view.Progress = s.progressLocked(playerID, q.ID)
		}
		filtered = append(filtered, view)
	}
	sortSliceBy(m, filtered)
	return encodeJSON(collection(slicePage(filtered, pp), len(filtered), pp))
}

func (s *rpgStore) taskClaim(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	result, err := s.claimQuest(valueString(m, "playerId"), valueString(m, "questId"), "player")
	if err != nil {
		return nil, err
	}
	return encodeJSON(result)
}

func (s *rpgStore) taskRefresh(_ context.Context, payload []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	today := dayKey(nowMillis())
	refreshed := 0
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
		refreshed++
	}
	s.expireBansLocked()
	s.persistStateLocked()
	return encodeJSON(map[string]any{"success": true, "refreshed": refreshed})
}

// --- order.* ------------------------------------------------------------------

func (s *rpgStore) orderList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]rpgOrder, 0, len(s.orders))
	for _, o := range s.orders {
		if q := valueString(m, "playerId"); q != "" && o.PlayerID != q {
			continue
		}
		if q := valueString(m, "status"); q != "" && o.Status != q {
			continue
		}
		filtered = append(filtered, o)
	}
	sortSliceBy(m, filtered)
	return encodeJSON(collection(slicePage(filtered, pp), len(filtered), pp))
}

func (s *rpgStore) orderShip(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	result, err := s.shipOrder(valueString(m, "orderNo"), "platform")
	if err != nil {
		return nil, err
	}
	return encodeJSON(result)
}

func (s *rpgStore) orderRefund(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	result, err := s.refundOrder(valueString(m, "orderNo"), "platform")
	if err != nil {
		return nil, err
	}
	return encodeJSON(result)
}

// --- economy.* ----------------------------------------------------------------

func (s *rpgStore) economySnapshot(_ context.Context, _ []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	online, banned := 0, 0
	gold, diamonds := int64(0), int64(0)
	items := map[string]int64{}
	for i := range s.players {
		p := &s.players[i]
		switch p.Status {
		case "online":
			online++
		case "banned":
			banned++
		}
		gold += p.Gold
		diamonds += p.Diamonds
		for id, count := range p.Inventory {
			items[id] += count
		}
	}
	orderStatuses := map[string]int{}
	unclaimed := 0
	for i := range s.mails {
		if !s.mails[i].Claimed && !s.mailExpiredLocked(&s.mails[i]) {
			unclaimed++
		}
	}
	for i := range s.orders {
		orderStatuses[s.orders[i].Status]++
	}
	claimable := 0
	for i := range s.progress {
		if s.progress[i].Status == "claimable" {
			claimable++
		}
	}
	return encodeJSON(map[string]any{
		"players": len(s.players), "online": online, "banned": banned,
		"gold": gold, "diamonds": diamonds,
		"orders": orderStatuses, "mailsUnclaimed": unclaimed,
		"items": items, "questsClaimable": claimable,
	})
}

func (s *rpgStore) economyAudit(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]rpgAuditRecord, 0, len(s.audit))
	for _, a := range s.audit {
		if q := valueString(m, "action"); q != "" && a.Action != q {
			continue
		}
		if q := valueString(m, "actor"); q != "" && a.Actor != q {
			continue
		}
		filtered = append(filtered, a)
	}
	return encodeJSON(collection(slicePage(filtered, pp), len(filtered), pp))
}

// --- report.generate -----------------------------------------------------------
//
// Task-mode executor mirroring the gameops-demo semantics: delay first so a
// cancelled task never records a run, then count the run in the world
// snapshot and fail on demand. runSeq persists across restarts so a retried
// invocation cannot re-hand out a reportId.

func (s *rpgStore) reportGenerate(ctx context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	reportRange := strOr(valueString(m, "range"), "daily")
	delayMs := 2000
	if _, ok := m["delayMs"]; ok {
		delayMs = valueInt(m, "delayMs")
	}
	if delayMs < 0 {
		delayMs = 0
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("report_generate_cancelled: %w", err)
	}
	timer := time.NewTimer(time.Duration(delayMs) * time.Millisecond)
	select {
	case <-ctx.Done():
		timer.Stop()
		return nil, fmt.Errorf("report_generate_cancelled: %w", ctx.Err())
	case <-timer.C:
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("report_generate_cancelled: %w", ctx.Err())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.reportRunSeq++
	gold, diamonds := int64(0), int64(0)
	for i := range s.players {
		gold += s.players[i].Gold
		diamonds += s.players[i].Diamonds
	}
	s.pushAuditLocked("system", "report.generate", reportRange, "success",
		fmt.Sprintf("run %d, %d players", s.reportRunSeq, len(s.players)))
	s.persistStateLocked()
	reportID := "report-rpg-" + itoa(s.reportRunSeq)
	return encodeJSON(map[string]any{
		"success":  true,
		"reportId": reportID,
		"runSeq":   s.reportRunSeq,
		"players":  len(s.players),
		"gold":     gold,
		"diamonds": diamonds,
		"range":    reportRange,
	})
}

func int64Or(v int, fallback int64) int64 {
	if v == 0 {
		return fallback
	}
	return int64(v)
}
