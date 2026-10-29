package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// executor is the E1-mirroring business logic bound to one FunctionContract.
// All state mutations go through store with write-then-read visibility.

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

func (s *gameOpsStore) playerByIDLocked(id string) *gameOpsPlayer {
	for i := range s.players {
		if s.players[i].ID == id {
			return &s.players[i]
		}
	}
	return nil
}

func (s *gameOpsStore) taskByIDLocked(id string) *gameOpsTask {
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			return &s.tasks[i]
		}
	}
	return nil
}

func (s *gameOpsStore) banByIDLocked(id string) *gameOpsBan {
	for i := range s.bans {
		if s.bans[i].ID == id {
			return &s.bans[i]
		}
	}
	return nil
}

func (s *gameOpsStore) orderByNoLocked(no string) *gameOpsOrder {
	for i := range s.orders {
		if s.orders[i].OrderNo == no {
			return &s.orders[i]
		}
	}
	return nil
}

func (s *gameOpsStore) activityByIDLocked(id string) *gameOpsActivity {
	for i := range s.activities {
		if s.activities[i].ID == id {
			return &s.activities[i]
		}
	}
	return nil
}

// --- player.* ---------------------------------------------------------------

func (s *gameOpsStore) playerList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]gameOpsPlayer, 0, len(s.players))
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

func (s *gameOpsStore) playerGet(_ context.Context, payload []byte) ([]byte, error) {
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

func (s *gameOpsStore) playerCreate(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	maxID := 10000
	for _, p := range s.players {
		if n := atoi(p.ID, 0); n > maxID {
			maxID = n
		}
	}
	id := intStr(maxID + 1)
	s.players = append([]gameOpsPlayer{{
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
	return encodeJSON(map[string]any{"id": id})
}

func (s *gameOpsStore) playerUpdate(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	return encodeJSON(map[string]any{"success": true})
}

func (s *gameOpsStore) playerDelete(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := valueString(m, "id")
	for i := range s.players {
		if s.players[i].ID == id {
			s.players = append(s.players[:i], s.players[i+1:]...)
			break
		}
	}
	return encodeJSON(map[string]any{"success": true})
}

func (s *gameOpsStore) playerBan(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := valueString(m, "playerId")
	p := s.playerByIDLocked(id)
	if p == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	p.Status = "banned"
	return encodeJSON(map[string]any{"success": true})
}

func (s *gameOpsStore) playerRecharge(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := valueString(m, "playerId")
	p := s.playerByIDLocked(id)
	if p == nil {
		return nil, fmt.Errorf("player_not_found")
	}
	amount := valueInt64(m, "amount")
	p.BalanceGold += amount
	return encodeJSON(map[string]any{"success": true, "amount": amount})
}

func (s *gameOpsStore) playerMail(_ context.Context, payload []byte) ([]byte, error) {
	// 只落操作记录；邮件正文由真实邮件服务投递（demo 收件箱不单独建模）。
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushOpLocked("admin", "player.mail", valueString(m, "playerId"), "success", nil)
	return encodeJSON(map[string]any{"success": true})
}

func (s *gameOpsStore) playerBatchBan(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	ids := stringSlice(toAnySlice(m["playerIds"]))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.players {
		if contains(ids, s.players[i].ID) {
			s.players[i].Status = "banned"
		}
	}
	return encodeJSON(map[string]any{"success": true, "count": len(ids)})
}

func (s *gameOpsStore) playerBatchUnban(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	ids := stringSlice(toAnySlice(m["playerIds"]))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.players {
		if contains(ids, s.players[i].ID) {
			s.players[i].Status = "active"
		}
	}
	return encodeJSON(map[string]any{"success": true, "count": len(ids)})
}

// --- mail_task.* ------------------------------------------------------------

func (s *gameOpsStore) mailTaskList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]gameOpsTask, 0, len(s.tasks))
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

func (s *gameOpsStore) mailTaskGet(_ context.Context, payload []byte) ([]byte, error) {
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
	return encodeJSON(t)
}

func (s *gameOpsStore) mailTaskCreate(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	taskType := valueString(m, "type")
	if taskType != "compensation" {
		taskType = "mail"
	}
	targetCount := 26
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
	// 兼容。邮件与全服/区服任务仍按既有口径（26 / 区服人数）。
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
	}
	task := gameOpsTask{
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
	s.tasks = append([]gameOpsTask{task}, s.tasks...)
	if approvalID != nil {
		s.pushOpLocked("admin", "compensation.grant", id, "pending_approval", approvalID)
	}
	out := map[string]any{"id": id}
	if approvalID != nil {
		out["approvalId"] = *approvalID
	}
	return encodeJSON(out)
}

func (s *gameOpsStore) mailTaskUpdate(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	return encodeJSON(map[string]any{"success": true})
}

func (s *gameOpsStore) mailTaskRetryFailed(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	return encodeJSON(map[string]any{"success": true, "retried": retried})
}

func (s *gameOpsStore) mailTaskCancel(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.taskByIDLocked(valueString(m, "id"))
	if t == nil {
		return nil, fmt.Errorf("task_not_found")
	}
	if t.Status == "sent" || t.Status == "completed" {
		return nil, fmt.Errorf("task_already_finished")
	}
	t.Status = "cancelled"
	s.pushOpLocked("admin", "mail_task.cancel", t.ID, "success", nil)
	return encodeJSON(map[string]any{"success": true})
}

// --- report.generate --------------------------------------------------------
//
// The only task-mode executor. It simulates an asynchronous report job: the
// handler returns after delayMs (default ~2s so the result is unknown to the
// caller until the task streams completion), records a run sequence in the
// store, and fails when `fail` is true. It honours context cancellation so a
// cancelled task stops waiting instead of completing late.
func (s *gameOpsStore) reportGenerate(ctx context.Context, payload []byte) ([]byte, error) {
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

	s.mu.Lock()
	defer s.mu.Unlock()
	s.reportRunSeq++
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

func (s *gameOpsStore) banList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]gameOpsBan, 0, len(s.bans))
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

func (s *gameOpsStore) banGet(_ context.Context, payload []byte) ([]byte, error) {
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
	return encodeJSON(b)
}

func (s *gameOpsStore) banCreate(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := strings.TrimSpace(valueString(m, "playerId"))
	duration := valueInt(m, "durationHours")
	if playerID == "" {
		return nil, fmt.Errorf("player_id_required")
	}
	if duration <= 0 {
		return nil, fmt.Errorf("duration_must_be_positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	player := s.playerByIDLocked(playerID)
	nickname := "玩家" + playerID + "号"
	serverID := strOr(valueString(m, "serverId"), "s1")
	if player != nil {
		nickname = player.Nickname
		serverID = player.ServerID
	}
	id := "ban-demo-" + intStr(len(s.bans)+1)
	now := nowMillis()
	record := gameOpsBan{
		ID: id, PlayerID: playerID, Nickname: nickname, ServerID: serverID,
		ApprovalID:    "approval-" + id,
		Reason:        strOr(valueString(m, "reason"), "risk"),
		DurationHours: duration,
		BannedAt:      millisISO(now),
		ExpiresAt:     millisISO(now + int64(duration)*3_600_000),
		Status:        "active", Operator: "admin", Approver: "admin",
	}
	s.bans = append([]gameOpsBan{record}, s.bans...)
	s.pushOpLocked("admin", "player.ban", "player "+playerID, "success", &record.ApprovalID)
	return encodeJSON(map[string]any{"id": id})
}

func (s *gameOpsStore) banUnban(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.banByIDLocked(valueString(m, "id"))
	if b == nil {
		return nil, fmt.Errorf("ban_not_found")
	}
	if b.Status != "active" {
		return nil, fmt.Errorf("ban_not_active")
	}
	b.Status = "lifted"
	s.pushOpLocked("admin", "player.unban", "player "+b.PlayerID, "success", nil)
	return encodeJSON(map[string]any{"success": true})
}

func (s *gameOpsStore) banExtend(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	extra := valueInt(m, "extraHours")
	if extra <= 0 {
		return nil, fmt.Errorf("duration_must_be_positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	return encodeJSON(map[string]any{"success": true, "extraHours": extra})
}

func (s *gameOpsStore) banBatchUnban(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	ids := stringSlice(toAnySlice(m["ids"]))
	s.mu.Lock()
	defer s.mu.Unlock()
	lifted := 0
	for i := range s.bans {
		if contains(ids, s.bans[i].ID) && s.bans[i].Status == "active" {
			s.bans[i].Status = "lifted"
			lifted++
		}
	}
	s.pushOpLocked("admin", "ban.batch_unban", countLabel(lifted)+" ban(s)", "success", nil)
	return encodeJSON(map[string]any{"success": true, "lifted": lifted})
}

// --- order.* ----------------------------------------------------------------

func (s *gameOpsStore) orderList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]gameOpsOrder, 0, len(s.orders))
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

func (s *gameOpsStore) orderGet(_ context.Context, payload []byte) ([]byte, error) {
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
	return encodeJSON(o)
}

func (s *gameOpsStore) orderRecredit(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	return encodeJSON(map[string]any{"success": true, "orderNo": o.OrderNo})
}

// --- activity.* -------------------------------------------------------------

func (s *gameOpsStore) activityList(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp := newPageParams(m)
	filtered := make([]gameOpsActivity, 0, len(s.activities))
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

func (s *gameOpsStore) activityGet(_ context.Context, payload []byte) ([]byte, error) {
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
	copy := *a
	copy.Versions = append([]gameOpsActivityVersion(nil), a.Versions...)
	history := make([]string, 0, len(copy.Versions))
	for i := len(copy.Versions) - 1; i >= 0; i-- {
		v := copy.Versions[i]
		history = append(history, "v"+intStr(v.Version)+" "+v.Note+" · "+v.Operator)
	}
	out := make(map[string]any)
	raw, _ := encodeJSON(copy)
	_ = decodeInto(raw, &out)
	out["versionHistory"] = strings.Join(history, "；")
	return encodeJSON(out)
}

func (s *gameOpsStore) activityCreate(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(valueString(m, "name"))
	if name == "" {
		return nil, fmt.Errorf("name_required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seq := len(s.activities) + 1
	id := "act-demo-" + intStr(seq)
	now := millisISO(nowMillis())
	s.activities = append([]gameOpsActivity{{
		ID: id, Name: name,
		Type:     strOr(valueString(m, "type"), "login_gift"),
		Timezone: strOr(valueString(m, "timezone"), "Asia/Shanghai"),
		StartAt:  now, EndAt: now, Status: "draft", CurrentVersion: 1,
		Versions: []gameOpsActivityVersion{{Version: 1, Note: "初版配置", PublishedAt: now, Operator: "admin"}},
	}}, s.activities...)
	return encodeJSON(map[string]any{"id": id})
}

func (s *gameOpsStore) activityPublish(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.activityByIDLocked(valueString(m, "id"))
	if a == nil {
		return nil, fmt.Errorf("activity_not_found")
	}
	version := len(a.Versions) + 1
	note := strOr(valueString(m, "note"), "调整奖励数值")
	a.Versions = append(a.Versions, gameOpsActivityVersion{
		Version: version, Note: note, PublishedAt: millisISO(nowMillis()), Operator: "admin",
	})
	a.CurrentVersion = version
	a.Status = "published"
	s.pushOpLocked("admin", "activity.publish", a.ID, "success", nil)
	return encodeJSON(map[string]any{"version": version})
}

func (s *gameOpsStore) activityActivate(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.activityByIDLocked(valueString(m, "id"))
	if a == nil {
		return nil, fmt.Errorf("activity_not_found")
	}
	if a.Status != "published" && a.Status != "rolled_back" {
		return nil, fmt.Errorf("activate_requires_published")
	}
	a.Status = "active"
	s.pushOpLocked("admin", "activity.activate", a.ID, "success", nil)
	return encodeJSON(map[string]any{"success": true})
}

func (s *gameOpsStore) activityEnd(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.activityByIDLocked(valueString(m, "id"))
	if a == nil {
		return nil, fmt.Errorf("activity_not_found")
	}
	if a.Status != "active" && a.Status != "published" {
		return nil, fmt.Errorf("end_requires_active")
	}
	a.Status = "ended"
	s.pushOpLocked("admin", "activity.end", a.ID, "success", nil)
	return encodeJSON(map[string]any{"success": true})
}

func (s *gameOpsStore) activityRollback(_ context.Context, payload []byte) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	target := valueInt(m, "targetVersion")
	s.mu.Lock()
	defer s.mu.Unlock()
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
	return encodeJSON(map[string]any{"success": true, "version": target})
}
