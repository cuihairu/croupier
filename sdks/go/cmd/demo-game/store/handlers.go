package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// The 34 game functions, ported from examples/gameops-demo executors.go.
// Read handlers keep the original RLock bodies; every mutation routes through
// (*Store).write for exactly-once semantics. Guard chains, error strings,
// receipt shapes and the {items,total,page,pageSize} list envelope are
// contract surface — keep them byte-identical to gameops-demo.

type pageParams struct {
	current  int
	pageSize int
	value    map[string]any
}

func newPageParams(m map[string]any) pageParams {
	return pageParams{
		current:  valueInt(m, "current"),
		pageSize: valueInt(m, "pageSize"),
		value:    m,
	}
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

// --- player.* ---------------------------------------------------------------

func (s *Store) PlayerList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]Player, 0, len(s.players))
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

func (s *Store) PlayerGet(_ context.Context, payload []byte) ([]byte, error) {
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
	return encodeJSON(*p)
}

func (s *Store) PlayerCreate(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opPlayerCreate, payload, func(m map[string]any) (any, error) {
		maxID := 10000
		for _, p := range s.players {
			if n := atoi(p.ID); n > maxID {
				maxID = n
			}
		}
		id := intStr(maxID + 1)
		s.players = append([]Player{{
			ID:          id,
			Nickname:    strOr(valueString(m, "nickname"), "玩家"+id),
			Character:   strOr(valueString(m, "character"), "角色"+id),
			ServerID:    strOr(valueString(m, "serverId"), "s1"),
			Level:       valueInt(m, "level"),
			VIP:         valueInt(m, "vip"),
			Status:      "active",
			BalanceGold: 0,
			LastLoginAt: millisISO(nowMillis()),
			CreatedAt:   millisISO(nowMillis()),
		}}, s.players...)
		if s.players[0].Level == 0 {
			s.players[0].Level = 1
		}
		return map[string]any{"id": id}, nil
	})
}

func (s *Store) PlayerUpdate(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opPlayerUpdate, payload, func(m map[string]any) (any, error) {
		p := s.playerByIDLocked(valueString(m, "id"))
		if p == nil {
			return nil, fmt.Errorf("player_not_found")
		}
		if q := valueString(m, "nickname"); q != "" {
			p.Nickname = q
		}
		if q := valueInt(m, "level"); q != 0 {
			p.Level = q
		}
		if q := valueInt(m, "vip"); q != 0 {
			p.VIP = q
		}
		return map[string]any{"success": true}, nil
	})
}

func (s *Store) PlayerDelete(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opPlayerDelete, payload, func(m map[string]any) (any, error) {
		id := valueString(m, "id")
		for i := range s.players {
			if s.players[i].ID == id {
				s.players = append(s.players[:i], s.players[i+1:]...)
				break
			}
		}
		return map[string]any{"success": true}, nil
	})
}

func (s *Store) PlayerBan(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opPlayerBan, payload, func(m map[string]any) (any, error) {
		p := s.playerByIDLocked(valueString(m, "playerId"))
		if p == nil {
			return nil, fmt.Errorf("player_not_found")
		}
		p.Status = "banned"
		return map[string]any{"success": true}, nil
	})
}

func (s *Store) PlayerRecharge(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opPlayerRecharge, payload, func(m map[string]any) (any, error) {
		p := s.playerByIDLocked(valueString(m, "playerId"))
		if p == nil {
			return nil, fmt.Errorf("player_not_found")
		}
		amount := valueInt64(m, "amount")
		p.BalanceGold += amount
		return map[string]any{"success": true, "amount": amount}, nil
	})
}

func (s *Store) PlayerBatchRecharge(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opPlayerBatchRecharge, payload, func(m map[string]any) (any, error) {
		ids := stringSlice(toAnySlice(m["playerIds"]))
		amount := valueInt64(m, "amount")
		if amount < 0 {
			return nil, fmt.Errorf("invalid_amount")
		}
		for i := range s.players {
			if contains(ids, s.players[i].ID) {
				s.players[i].BalanceGold += amount
			}
		}
		return map[string]any{
			"success": true,
			"amount":  amount,
			"count":   len(ids),
			"total":   amount * int64(len(ids)),
		}, nil
	})
}

func (s *Store) PlayerMail(_ context.Context, payload []byte) ([]byte, error) {
	// 只落操作记录；邮件正文由真实邮件服务投递（demo 收件箱不单独建模）。
	return s.write(opPlayerMail, payload, func(m map[string]any) (any, error) {
		s.pushOpLocked("admin", "player.mail", valueString(m, "playerId"), "success", nil)
		return map[string]any{"success": true}, nil
	})
}

func (s *Store) PlayerBatchBan(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opPlayerBatchBan, payload, func(m map[string]any) (any, error) {
		ids := stringSlice(toAnySlice(m["playerIds"]))
		for i := range s.players {
			if contains(ids, s.players[i].ID) {
				s.players[i].Status = "banned"
			}
		}
		return map[string]any{"success": true, "count": len(ids)}, nil
	})
}

func (s *Store) PlayerBatchUnban(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opPlayerBatchUnban, payload, func(m map[string]any) (any, error) {
		ids := stringSlice(toAnySlice(m["playerIds"]))
		for i := range s.players {
			if contains(ids, s.players[i].ID) {
				s.players[i].Status = "active"
			}
		}
		return map[string]any{"success": true, "count": len(ids)}, nil
	})
}

// --- mail_task.* ------------------------------------------------------------

func (s *Store) MailTaskList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if q := valueString(m, "type"); q != "" && t.Type != q {
			continue
		}
		if q := valueString(m, "status"); q != "" && t.Status != q {
			continue
		}
		filtered = append(filtered, t)
	}
	sortSliceBy(m, filtered)
	return encodeJSON(collection(slicePage(filtered, pp), len(filtered), pp))
}

func (s *Store) MailTaskGet(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	t := s.taskByIDLocked(valueString(m, "id"))
	if t == nil {
		return []byte("null"), nil
	}
	return encodeJSON(*t)
}

func (s *Store) MailTaskCreate(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opMailTaskCreate, payload, func(m map[string]any) (any, error) {
		taskType := valueString(m, "type")
		if taskType != "compensation" {
			taskType = "mail"
		}
		// 补偿默认单目标（与 Web demo 口径 1 一致），邮件默认 26；多人补偿由
		// 下方 targetPlayerIds 唯一数覆盖（D2 批量场景）。
		targetCount := 26
		if taskType == "compensation" {
			targetCount = 1
		}
		var perAmount *int64
		if taskType == "compensation" {
			perAmount = int64Ptr(valueInt64(m, "perAmount"))
		}
		id := "task-demo-" + intStr(len(s.tasks)+1)
		var approvalID *string
		if taskType == "compensation" {
			approvalID = strPtr("approval-" + id)
		}
		status := "sent"
		if taskType == "compensation" {
			status = "pending_approval"
		}
		receiverRange := valueString(m, "receiverRange")
		if receiverRange == "" {
			receiverRange = "all"
		}
		// 指定玩家补偿支持多人目标（D2 批量补偿场景）：targetPlayerIds 为目标
		// 快照数组，按唯一玩家数计算目标人数与补偿总量；单值 targetPlayerId 保留
		// 兼容。审核发现：players 范围但无任何目标标识（数组与单值皆空）时
		// fail-closed 拒绝——不允许落「声明 1 个目标但无目标标识」的失真快照。
		// 邮件按既有口径 26（全服/区服），补偿默认单目标 1（Web demo 口径）。
		targetPlayerIDs := make([]string, 0)
		if taskType == "compensation" && receiverRange == "players" {
			if ids := valueStringSlice(m, "targetPlayerIds"); len(ids) > 0 {
				targetPlayerIDs = uniqueStrings(ids)
			} else if pid := valueString(m, "targetPlayerId"); pid != "" {
				targetPlayerIDs = []string{pid}
			}
			if len(targetPlayerIDs) > 0 {
				targetCount = len(targetPlayerIDs)
			}
			if len(targetPlayerIDs) == 0 {
				return nil, fmt.Errorf("指定玩家补偿必须选择至少一名玩家")
			}
		}
		task := Task{
			ID: id, Type: taskType,
			Title:           strOr(valueString(m, "title"), "未命名任务"),
			Detail:          valueString(m, "detail"),
			ReceiverRange:   receiverRange,
			TargetServerID:  optionalString(receiverRange == "server", valueString(m, "targetServerId"), "s1"),
			TargetPlayerID:  optionalString(receiverRange == "players" && valueString(m, "targetPlayerId") != "", valueString(m, "targetPlayerId"), ""),
			TargetPlayerIDs: targetPlayerIDs,
			TargetOrderNo:   optionalString(valueString(m, "targetOrderNo") != "", valueString(m, "targetOrderNo"), ""),
			TargetCount:     targetCount,
			PerAmount:       perAmount,
			TotalAmount:     multiplyPtr(perAmount, targetCount),
			Currency:        optionalString(taskType == "compensation", "金", ""),
			Status:          status,
			SentCount:       ternaryInt(taskType == "mail", targetCount, 0),
			Operator:        "admin",
			ApprovalID:      approvalID,
			CreatedAt:       millisISO(nowMillis()),
		}
		s.tasks = append([]Task{task}, s.tasks...)
		if approvalID != nil {
			s.pushOpLocked("admin", "compensation.grant", id, "pending_approval", approvalID)
		}
		out := map[string]any{"id": id}
		if approvalID != nil {
			out["approvalId"] = *approvalID
		}
		return out, nil
	})
}

func (s *Store) MailTaskUpdate(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opMailTaskUpdate, payload, func(m map[string]any) (any, error) {
		t := s.taskByIDLocked(valueString(m, "id"))
		if t == nil {
			return nil, fmt.Errorf("task_not_found")
		}
		if q := valueString(m, "title"); q != "" {
			t.Title = q
		}
		if q := valueString(m, "detail"); q != "" {
			t.Detail = q
		}
		return map[string]any{"success": true}, nil
	})
}

func (s *Store) MailTaskRetryFailed(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opMailTaskRetryFailed, payload, func(m map[string]any) (any, error) {
		t := s.taskByIDLocked(valueString(m, "id"))
		if t == nil {
			return nil, fmt.Errorf("task_not_found")
		}
		if t.FailedCount == 0 {
			return nil, fmt.Errorf("no_failed_targets")
		}
		retried := t.FailedCount
		t.SentCount += t.FailedCount
		t.FailedCount = 0
		t.Status = "sent"
		s.pushOpLocked("admin", "mail_task.retry_failed", t.ID, "success", nil)
		return map[string]any{"success": true, "retried": retried}, nil
	})
}

func (s *Store) MailTaskCancel(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opMailTaskCancel, payload, func(m map[string]any) (any, error) {
		t := s.taskByIDLocked(valueString(m, "id"))
		if t == nil {
			return nil, fmt.Errorf("task_not_found")
		}
		if t.Status == "sent" || t.Status == "completed" {
			return nil, fmt.Errorf("task_already_finished")
		}
		t.Status = "cancelled"
		s.pushOpLocked("admin", "mail_task.cancel", t.ID, "success", nil)
		return map[string]any{"success": true}, nil
	})
}

// --- report.generate --------------------------------------------------------
//
// The only task-mode executor. It simulates an asynchronous report job: the
// handler returns after delayMs (default ~2s so the result is unknown to the
// caller until the task streams completion), records a run sequence in the
// store (persisted through the backend meta row, so a re-delivered task after
// a restart does not re-hand out a reportId), and fails when `fail` is true.
// It honours context cancellation so a cancelled task stops waiting instead
// of completing late. Unlike the sync mutations it does not run in a write
// transaction — the platform task layer owns task-level idempotency; the run
// counter persistence is this handler's own restart guarantee.
func (s *Store) ReportGenerate(ctx context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	reportRange := strOr(valueString(m, "range"), "daily")
	// Default the delay only when the field is absent; an explicit delayMs=0
	// means "no delay" (tests use this so the scenario suite stays fast).
	delayMs := 2000
	if _, ok := m["delayMs"]; ok {
		delayMs = valueInt(m, "delayMs")
	}
	if delayMs < 0 {
		delayMs = 0
	}
	// Delay first so a cancelled task never records a run (the observable
	// result-unknown window is the time between StartTask and completion).
	// Check cancellation up front: with delayMs=0 the timer is already ready and
	// a select between ctx.Done() and timer.C is random, so a pre-cancelled
	// context could otherwise take the timer branch and record a run.
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
	// 取消与 timer 同时就绪的窗口已闭合：timer 分支命中后若 ctx 已取消，
	// 取消优先——不记录 run（delayMs=0 与取消竞态不再随机记录，闭合「取消
	// 绝不记录」语义）。
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("report_generate_cancelled: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.reportRunSeq++
	s.meta.ReportRunSeq = s.reportRunSeq
	if err := s.backend.UpdateMeta(s.metaLocked()); err != nil {
		return nil, fmt.Errorf("persist report run counter: %w", err)
	}
	if valueBool(m, "fail") {
		return nil, fmt.Errorf("report_generate_failed: simulated failure for range %s", reportRange)
	}
	id := "report-demo-" + intStr(s.reportRunSeq)
	s.pushOpLocked("admin", "report.generate", id, "success", nil)
	return encodeJSON(map[string]any{
		"success":  true,
		"reportId": id,
		"runSeq":   s.reportRunSeq,
		"rows":     42,
		"range":    reportRange,
	})
}

// --- ban.* ------------------------------------------------------------------

func (s *Store) BanList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]Ban, 0, len(s.bans))
	for _, b := range s.bans {
		if q := valueString(m, "nickname"); q != "" && !strings.Contains(b.Nickname, q) {
			continue
		}
		if q := valueString(m, "status"); q != "" && b.Status != q {
			continue
		}
		if q := valueString(m, "reason"); q != "" && b.Reason != q {
			continue
		}
		filtered = append(filtered, b)
	}
	sortSliceBy(m, filtered)
	return encodeJSON(collection(slicePage(filtered, pp), len(filtered), pp))
}

func (s *Store) BanGet(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := s.banByIDLocked(valueString(m, "id"))
	if b == nil {
		return []byte("null"), nil
	}
	return encodeJSON(*b)
}

func (s *Store) BanCreate(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opBanCreate, payload, func(m map[string]any) (any, error) {
		playerID := strings.TrimSpace(valueString(m, "playerId"))
		duration := valueInt(m, "durationHours")
		if playerID == "" {
			return nil, fmt.Errorf("player_id_required")
		}
		if duration <= 0 {
			return nil, fmt.Errorf("duration_must_be_positive")
		}
		player := s.playerByIDLocked(playerID)
		nickname := "玩家" + playerID + "号"
		serverID := strOr(valueString(m, "serverId"), "s1")
		if player != nil {
			nickname = player.Nickname
			serverID = player.ServerID
		}
		id := "ban-demo-" + intStr(len(s.bans)+1)
		now := nowMillis()
		record := Ban{
			ID: id, PlayerID: playerID, Nickname: nickname, ServerID: serverID,
			ApprovalID:    "approval-" + id,
			Reason:        strOr(valueString(m, "reason"), "risk"),
			DurationHours: duration,
			BannedAt:      millisISO(now),
			ExpiresAt:     millisISO(now + int64(duration)*3_600_000),
			Status:        "active", Operator: "admin", Approver: "admin",
		}
		s.bans = append([]Ban{record}, s.bans...)
		s.pushOpLocked("admin", "player.ban", "player "+playerID, "success", &record.ApprovalID)
		return map[string]any{"id": id}, nil
	})
}

func (s *Store) BanUnban(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opBanUnban, payload, func(m map[string]any) (any, error) {
		b := s.banByIDLocked(valueString(m, "id"))
		if b == nil {
			return nil, fmt.Errorf("ban_not_found")
		}
		if b.Status != "active" {
			return nil, fmt.Errorf("ban_not_active")
		}
		b.Status = "lifted"
		s.pushOpLocked("admin", "player.unban", "player "+b.PlayerID, "success", nil)
		return map[string]any{"success": true}, nil
	})
}

func (s *Store) BanExtend(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opBanExtend, payload, func(m map[string]any) (any, error) {
		extra := valueInt(m, "extraHours")
		if extra <= 0 {
			return nil, fmt.Errorf("duration_must_be_positive")
		}
		b := s.banByIDLocked(valueString(m, "id"))
		if b == nil {
			return nil, fmt.Errorf("ban_not_found")
		}
		if b.Status != "active" {
			return nil, fmt.Errorf("ban_not_active")
		}
		b.DurationHours += extra
		expire, err := timeParseRFC3339(b.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("invalid_expires_at: %w", err)
		}
		b.ExpiresAt = millisISO(expire + int64(extra)*3_600_000)
		s.pushOpLocked("admin", "ban.extend", b.ID, "success", nil)
		return map[string]any{"success": true, "extraHours": extra}, nil
	})
}

func (s *Store) BanBatchUnban(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opBanBatchUnban, payload, func(m map[string]any) (any, error) {
		ids := stringSlice(toAnySlice(m["ids"]))
		lifted := 0
		for i := range s.bans {
			if contains(ids, s.bans[i].ID) && s.bans[i].Status == "active" {
				s.bans[i].Status = "lifted"
				lifted++
			}
		}
		s.pushOpLocked("admin", "ban.batch_unban", countLabel(lifted)+" ban(s)", "success", nil)
		return map[string]any{"success": true, "lifted": lifted}, nil
	})
}

// --- order.* ----------------------------------------------------------------

func (s *Store) OrderList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]Order, 0, len(s.orders))
	for _, o := range s.orders {
		if q := valueString(m, "nickname"); q != "" && !strings.Contains(o.Nickname, q) {
			continue
		}
		if q := valueString(m, "channel"); q != "" && o.Channel != q {
			continue
		}
		if q := valueString(m, "reconciliation"); q != "" && o.Reconciliation != q {
			continue
		}
		filtered = append(filtered, o)
	}
	sortSliceBy(m, filtered)
	return encodeJSON(collection(slicePage(filtered, pp), len(filtered), pp))
}

func (s *Store) OrderGet(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	o := s.orderByNoLocked(valueString(m, "orderNo"))
	if o == nil {
		return []byte("null"), nil
	}
	return encodeJSON(*o)
}

func (s *Store) OrderRecredit(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opOrderRecredit, payload, func(m map[string]any) (any, error) {
		o := s.orderByNoLocked(valueString(m, "orderNo"))
		if o == nil {
			return nil, fmt.Errorf("order_not_found")
		}
		if o.GameCredit == "credited" {
			return nil, fmt.Errorf("already_credited")
		}
		if o.GameCredit != "missing" {
			return nil, fmt.Errorf("credit_not_applicable")
		}
		ref := "credit-re-" + o.OrderNo
		now := millisISO(nowMillis())
		o.GameCredit = "credited"
		o.Status = "credited"
		o.Reconciliation = "match"
		o.GameCreditRef = &ref
		o.CreditedAt = &now
		s.pushOpLocked("admin", "order.recredit", o.OrderNo, "success", nil)
		return map[string]any{"success": true, "orderNo": o.OrderNo}, nil
	})
}

// --- activity.* -------------------------------------------------------------

func (s *Store) ActivityList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]Activity, 0, len(s.activities))
	for _, a := range s.activities {
		if q := valueString(m, "name"); q != "" && !strings.Contains(a.Name, q) {
			continue
		}
		if q := valueString(m, "status"); q != "" && a.Status != q {
			continue
		}
		if q := valueString(m, "type"); q != "" && a.Type != q {
			continue
		}
		filtered = append(filtered, a)
	}
	sortSliceBy(m, filtered)
	return encodeJSON(collection(slicePage(filtered, pp), len(filtered), pp))
}

func (s *Store) ActivityGet(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	a := s.activityByIDLocked(valueString(m, "id"))
	if a == nil {
		return []byte("null"), nil
	}
	out := make(map[string]any)
	raw, _ := encodeJSON(*a)
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode activity detail: %w", err)
	}
	history := make([]string, 0, len(a.Versions))
	for i := len(a.Versions) - 1; i >= 0; i-- {
		v := a.Versions[i]
		history = append(history, "v"+intStr(v.Version)+" "+v.Note+" · "+v.Operator)
	}
	out["versionHistory"] = strings.Join(history, "；")
	return encodeJSON(out)
}

func (s *Store) ActivityCreate(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opActivityCreate, payload, func(m map[string]any) (any, error) {
		name := strings.TrimSpace(valueString(m, "name"))
		if name == "" {
			return nil, fmt.Errorf("name_required")
		}
		seq := len(s.activities) + 1
		id := "act-demo-" + intStr(seq)
		now := millisISO(nowMillis())
		s.activities = append([]Activity{{
			ID: id, Name: name,
			Type:     strOr(valueString(m, "type"), "login_gift"),
			Timezone: strOr(valueString(m, "timezone"), "Asia/Shanghai"),
			StartAt:  now, EndAt: now, Status: "draft", CurrentVersion: 1,
			Versions: []ActivityVersion{{Version: 1, Note: "初版配置", PublishedAt: now, Operator: "admin"}},
		}}, s.activities...)
		return map[string]any{"id": id}, nil
	})
}

func (s *Store) ActivityPublish(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opActivityPublish, payload, func(m map[string]any) (any, error) {
		a := s.activityByIDLocked(valueString(m, "id"))
		if a == nil {
			return nil, fmt.Errorf("activity_not_found")
		}
		version := len(a.Versions) + 1
		note := strOr(valueString(m, "note"), "调整奖励数值")
		a.Versions = append(a.Versions, ActivityVersion{
			Version: version, Note: note, PublishedAt: millisISO(nowMillis()), Operator: "admin",
		})
		a.CurrentVersion = version
		a.Status = "published"
		s.pushOpLocked("admin", "activity.publish", a.ID, "success", nil)
		return map[string]any{"version": version}, nil
	})
}

func (s *Store) ActivityActivate(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opActivityActivate, payload, func(m map[string]any) (any, error) {
		a := s.activityByIDLocked(valueString(m, "id"))
		if a == nil {
			return nil, fmt.Errorf("activity_not_found")
		}
		if a.Status != "published" && a.Status != "rolled_back" {
			return nil, fmt.Errorf("activate_requires_published")
		}
		a.Status = "active"
		s.pushOpLocked("admin", "activity.activate", a.ID, "success", nil)
		return map[string]any{"success": true}, nil
	})
}

func (s *Store) ActivityEnd(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opActivityEnd, payload, func(m map[string]any) (any, error) {
		a := s.activityByIDLocked(valueString(m, "id"))
		if a == nil {
			return nil, fmt.Errorf("activity_not_found")
		}
		if a.Status != "active" && a.Status != "published" {
			return nil, fmt.Errorf("end_requires_active")
		}
		a.Status = "ended"
		s.pushOpLocked("admin", "activity.end", a.ID, "success", nil)
		return map[string]any{"success": true}, nil
	})
}

func (s *Store) ActivityRollback(_ context.Context, payload []byte) ([]byte, error) {
	return s.write(opActivityRollback, payload, func(m map[string]any) (any, error) {
		target := valueInt(m, "targetVersion")
		a := s.activityByIDLocked(valueString(m, "id"))
		if a == nil {
			return nil, fmt.Errorf("activity_not_found")
		}
		exists := false
		for _, v := range a.Versions {
			if v.Version == target {
				exists = true
				break
			}
		}
		if !exists {
			return nil, fmt.Errorf("target_version_not_found")
		}
		a.Status = "rolled_back"
		a.CurrentVersion = target
		s.pushOpLocked("admin", "activity.rollback", a.ID, "success", nil)
		return map[string]any{"success": true, "version": target}, nil
	})
}
