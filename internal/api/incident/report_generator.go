package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"gorm.io/gorm"
)

// 报告生成与分发（docs/design/incident-reports.md §6）：调度器 Kind=
// incident_report 经 scheduler.LocalRunner 触发（server-local 执行，不经
// agent 派发链；svc 布线由 cmd/server 注入）。流程 = 聚合（§5 口径）→
// 类别分级（固定口径，不配置化）→ upsert incident_reports → 分发（站内
// 链首 + 外部链可选降级）→ PushStatus 写回。

// 分级固定阈值（§6 v1 口径）：环比翻倍 / 复发率超 20% → warn；连续两期
// 恶化或含 critical 未解决事故 → critical。
const (
	escalateDoubleFactor  = 2.0
	escalateRecurrencePct = 20.0
)

// ReportCategorySlice 单类别分片（分级与分发的最小单元；落库 payload 组成
// 部分，重推按存库值原样重发不改写）。
type ReportCategorySlice struct {
	CategoryID         uint    `json:"categoryId"`
	Slug               string  `json:"slug,omitempty"`
	Name               string  `json:"name,omitempty"`
	Leader             string  `json:"leader,omitempty"`
	Count              int64   `json:"count"`
	PrevCount          int64   `json:"prevCount"`
	RecurrencePct      float64 `json:"recurrencePct"`      // 0-100；空样本 0
	UnresolvedCritical int64   `json:"unresolvedCritical"` // 期内 critical 未解决数
	Level              string  `json:"level"`              // info|warn|critical
}

// ReportPayload 是 incident_reports.Payload 的顶层结构：完整 summary +
// 类别分片分级 + 全报告最高级。
type ReportPayload struct {
	Summary    *ReportSummaryResponse `json:"summary"`
	Categories []ReportCategorySlice  `json:"categories"`
	Level      string                 `json:"level"` // info|warn|critical
}

// PushStatusEntry 分发回执（§6 PushStatus 数组元素）：每片一条，channel
// 为外部链收尾出口（herald/noop/…）或 skipped（未配 leader / 出口未布线）。
type PushStatusEntry struct {
	Slug     string    `json:"slug"`
	Leader   string    `json:"leader,omitempty"`
	Channel  string    `json:"channel"`
	PushedAt time.Time `json:"pushedAt"`
	EventID  string    `json:"eventId"`
	OK       bool      `json:"ok"`
	Error    string    `json:"error,omitempty"`
}

// GenerateReport 生成并分发一期报表（periodType: week|month|quarter|year；
// periodKey 空 = 当前周期）。幂等：同档同期 upsert 覆盖；外部出口按确定
// 性 event_id 折叠重发。
func (s *Service) GenerateReport(ctx context.Context, periodType, periodKey string) (*model.IncidentReport, error) {
	now := time.Now()
	p, err := resolvePeriod(periodType, periodKey, now)
	if err != nil {
		return nil, err
	}
	summary, err := s.ReportSummary(ctx, p.Type, p.Key)
	if err != nil {
		return nil, err
	}
	pieces, err := s.buildCategorySlices(ctx, p, now)
	if err != nil {
		return nil, err
	}
	level := model.MessageLevelInfo
	for i := range pieces {
		if sliceLevelRank(pieces[i].Level) > sliceLevelRank(level) {
			level = pieces[i].Level
		}
	}
	payload := ReportPayload{Summary: summary, Categories: pieces, Level: level}
	row, err := s.upsertReportRow(ctx, p.Type, p.Key, payload, now)
	if err != nil {
		return nil, err
	}
	status := s.distribute(ctx, p.Type, p.Key, &payload, now)
	if err := s.writePushStatus(ctx, row.ID, status); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(status)
	row.PushStatus = model.JSON(raw)
	return row, nil
}

// Run implements scheduler.LocalRunner：调度入口（task_schedules.Kind=
// incident_report）。Payload {"period":"week"}；缺 period 或档位非法报错
// （调度器按连续失败计数处理，达上限进 dead_letter）。
func (s *Service) Run(ctx context.Context, sched *model.TaskSchedule) error {
	if sched == nil {
		return errors.New("incident report runner: nil schedule")
	}
	var p struct {
		Period string `json:"period"`
	}
	if len(sched.Payload) > 0 {
		if err := json.Unmarshal(sched.Payload, &p); err != nil {
			return fmt.Errorf("incident report runner: parse payload: %w", err)
		}
	}
	if p.Period == "" {
		return errors.New("incident report runner: schedule payload missing period")
	}
	row, err := s.GenerateReport(ctx, p.Period, "")
	if err != nil {
		return fmt.Errorf("incident report runner: %w", err)
	}
	slog.InfoContext(ctx, "incident report generated", "schedule", sched.Name,
		"period", row.PeriodType, "periodKey", row.PeriodStart)
	return nil
}

// RepushReport 手动重推（§6：按存库 payload 同 event_id 重发，出口侧幂等
// 折叠；不改 payload）。回执覆盖写回 PushStatus。
func (s *Service) RepushReport(ctx context.Context, id uint) (*model.IncidentReport, error) {
	var row model.IncidentReport
	if err := s.svcCtx.DB.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	var payload ReportPayload
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return nil, errorx.NewBadRequest("报表 payload 损坏: " + err.Error())
	}
	if payload.Summary == nil {
		return nil, errorx.NewBadRequest("报表 payload 缺少 summary，无法重推")
	}
	status := s.distribute(ctx, row.PeriodType, row.PeriodStart, &payload, time.Now())
	if err := s.writePushStatus(ctx, row.ID, status); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(status)
	row.PushStatus = model.JSON(raw)
	return &row, nil
}

// ---- 分类分级 ----

// buildCategorySlices 聚合每类别的计数/环比/复发率/未解决 critical，并按
// 固定口径分级。计数含 bugs（与 summary 口径一致）。
func (s *Service) buildCategorySlices(ctx context.Context, p *resolvedPeriod, now time.Time) ([]ReportCategorySlice, error) {
	cur, err := s.periodStats(ctx, p.Start, p.End, now)
	if err != nil {
		return nil, err
	}
	prevShare, err := s.categoryShare(ctx, p.PrevStart, p.PrevEnd, cur.Total)
	if err != nil {
		return nil, err
	}
	prevByID := make(map[uint]int64, len(prevShare))
	for _, sh := range prevShare {
		prevByID[sh.CategoryID] = sh.Count
	}
	// 复发率分母/分子按类别：链首来自本期+期前窗口，分母=期内 resolved。
	rows := append(append([]incidentAggRow{}, cur.rows...), s.preWindowRows(ctx, p.Start)...)
	heads := recurrenceHeads(rows, p.Start, p.End)
	recurByCat := map[uint]int64{}
	for _, h := range heads {
		recurByCat[h.CategoryID]++
	}
	resolvedByCat := map[uint]int64{}
	for _, r := range cur.rows {
		if r.ResolvedAt != nil {
			resolvedByCat[r.CategoryID]++
		}
	}
	// 期内 critical 未解决按类别计数。
	type critRow struct {
		CategoryID uint
	}
	crit := []critRow{}
	if err := s.svcCtx.DB.WithContext(ctx).Model(&model.Incident{}).
		Select("category_id").
		Where("detected_at >= ? AND detected_at < ?", p.Start, p.End).
		Where("severity = ? AND status <> ?", model.IncidentSeverityCritical, model.IncidentStatusResolved).
		Limit(reportScanLimit).Scan(&crit).Error; err != nil {
		return nil, err
	}
	critByCat := map[uint]int64{}
	for _, r := range crit {
		critByCat[r.CategoryID]++
	}
	// leader/slug/name 索引（停用行也参与——报表面向存量类别）。
	cats, err := model.NewIncidentCategoryModel(s.svcCtx.DB).List(ctx, false)
	if err != nil {
		return nil, err
	}
	catByID := make(map[uint]model.IncidentCategory, len(cats))
	for _, c := range cats {
		catByID[c.ID] = c
	}
	// 计数按类别（incidents + bugs，与 categoryShare 同口径复用）。
	curShare, err := s.categoryShare(ctx, p.Start, p.End, cur.Total)
	if err != nil {
		return nil, err
	}
	pieces := make([]ReportCategorySlice, 0, len(curShare))
	for _, sh := range curShare {
		piece := ReportCategorySlice{
			CategoryID:         sh.CategoryID,
			Slug:               sh.Slug,
			Name:               sh.Name,
			Count:              sh.Count,
			PrevCount:          prevByID[sh.CategoryID],
			UnresolvedCritical: critByCat[sh.CategoryID],
		}
		if den := resolvedByCat[sh.CategoryID]; den > 0 {
			piece.RecurrencePct = float64(recurByCat[sh.CategoryID]) / float64(den) * 100
		}
		if c, ok := catByID[sh.CategoryID]; ok {
			piece.Leader = c.Leader
			if piece.Name == "" {
				piece.Name = c.Name
			}
			if piece.Slug == "" {
				piece.Slug = c.Slug
			}
		}
		classifySlice(ctx, s, p, &piece)
		pieces = append(pieces, piece)
	}
	return pieces, nil
}

// classifySlice 固定口径分级（§6）：warn=环比×2（基数>0 才有意义）或复发
// 率>20%；critical=含 critical 未解决事故或连续两期恶化（存库历史：
// cur > prev > prev-prev，缺任一期历史不判）。级别只升不降。
func classifySlice(ctx context.Context, s *Service, p *resolvedPeriod, piece *ReportCategorySlice) {
	level := model.MessageLevelInfo
	if piece.PrevCount > 0 && piece.Count > 0 &&
		float64(piece.Count) >= float64(piece.PrevCount)*escalateDoubleFactor {
		level = model.MessageLevelWarn
	}
	if piece.RecurrencePct > escalateRecurrencePct {
		level = model.MessageLevelWarn
	}
	if piece.UnresolvedCritical > 0 {
		level = model.MessageLevelCritical
	}
	if worsened, err := s.worsenedTwoPeriods(ctx, p.Type, p.Key, piece.Slug, piece.Count); err == nil && worsened {
		level = model.MessageLevelCritical
	} else if err != nil {
		slog.WarnContext(ctx, "incident report: read history for escalation failed", "error", err)
	}
	piece.Level = level
}

// worsenedTwoPeriods 连续两期恶化判定：读同档已存报表（period_start 早于
// 本期，倒序取两期），该类别 cur > prev > prev-prev。缺任一期历史 → false
// （首期报表无从判趋势）。
func (s *Service) worsenedTwoPeriods(ctx context.Context, periodType, periodKey, slug string, curCount int64) (bool, error) {
	if slug == "" {
		return false, nil
	}
	var rows []model.IncidentReport
	err := s.svcCtx.DB.WithContext(ctx).
		Where("period_type = ? AND period_start < ?", periodType, periodKey).
		Order("period_start DESC").Limit(2).Find(&rows).Error
	if err != nil {
		return false, err
	}
	if len(rows) < 2 {
		return false, nil
	}
	counts := make([]int64, 0, 2)
	for _, r := range rows {
		var payload ReportPayload
		if err := json.Unmarshal(r.Payload, &payload); err != nil {
			return false, nil // 历史 payload 损坏按无历史处理，不阻塞生成
		}
		for _, c := range payload.Categories {
			if c.Slug == slug {
				counts = append(counts, c.Count)
				break
			}
		}
	}
	if len(counts) < 2 {
		return false, nil
	}
	return curCount > counts[0] && counts[0] > counts[1], nil
}

// ---- 落库 ----

// upsertReportRow 同档同期 upsert（唯一索引 uidx_incident_reports_period；
// 并发双写兜底冲突转查询）。
func (s *Service) upsertReportRow(ctx context.Context, periodType, periodKey string, payload ReportPayload, now time.Time) (*model.IncidentReport, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	db := s.svcCtx.DB.WithContext(ctx)
	var row model.IncidentReport
	err = db.Where("period_type = ? AND period_start = ?", periodType, periodKey).First(&row).Error
	if err == nil {
		updates := map[string]interface{}{
			"payload":      model.JSON(raw),
			"generated_at": now,
			"report_kind":  model.ReportKindSummary,
		}
		if err := db.Model(&model.IncidentReport{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		row.Payload, row.GeneratedAt, row.ReportKind = model.JSON(raw), now, model.ReportKindSummary
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row = model.IncidentReport{
		PeriodType: periodType, PeriodStart: periodKey,
		ReportKind: model.ReportKindSummary, Payload: model.JSON(raw), GeneratedAt: now,
	}
	if err := db.Create(&row).Error; err != nil {
		if isUniqueViolation(err) {
			if ferr := db.Where("period_type = ? AND period_start = ?", periodType, periodKey).First(&row).Error; ferr == nil {
				return &row, nil
			}
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) writePushStatus(ctx context.Context, id uint, status []PushStatusEntry) error {
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return s.svcCtx.DB.WithContext(ctx).Model(&model.IncidentReport{}).
		Where("id = ?", id).Update("push_status", model.JSON(raw)).Error
}

// ---- 分发 ----

// distribute 执行分发（§6）：站内经 MessageSink 直写（总聚合落默认接收
// 组、分片落 leader 账号——类别→leader 映射只有生成器可见），外部经出口
// 链 DispatchExternal（target=category-leader:<slug>）。每片每渠道一条
// PushStatus；未配 leader 的分片记 skipped(no leader) 不投——数据仍在总
// 聚合与面板，不落空。重推同 event_id：站内 upsert 折叠、出口侧 dedup。
func (s *Service) distribute(ctx context.Context, periodType, periodKey string, payload *ReportPayload, now time.Time) []PushStatusEntry {
	base := eventIDBase(periodType, periodKey)
	status := make([]PushStatusEntry, 0, 2*len(payload.Categories)+2)

	// 总聚合片：站内默认接收组（管理员聚合）+ 外部默认受众。
	aggEv := s.aggregateEvent(base, periodType, periodKey, payload, now)
	status = append(status, s.deliverSlice(ctx, sliceRef{
		slug: "", leader: "", level: payload.Level,
		title: aggEv.Title, body: aggEv.Body, eventID: aggEv.EventID,
		scope: map[string]any{}, ev: &aggEv,
	}, now)...)

	for i := range payload.Categories {
		piece := &payload.Categories[i]
		if piece.Count <= 0 {
			continue // 零量类别不投分片（leader 不被零事件打扰；数据在总聚合）
		}
		if piece.Leader == "" {
			status = append(status, PushStatusEntry{
				Slug: piece.Slug, Channel: "skipped", PushedAt: now,
				EventID: base + ":" + piece.Slug, OK: false, Error: "no leader",
			})
			continue
		}
		ev := s.sliceEvent(base, periodType, periodKey, piece, now)
		status = append(status, s.deliverSlice(ctx, sliceRef{
			slug: piece.Slug, leader: piece.Leader, level: piece.Level,
			title: ev.Title, body: ev.Body, eventID: ev.EventID,
			scope: map[string]any{"categories": []string{piece.Slug}}, ev: &ev,
		}, now)...)
	}
	return status
}

// sliceRef 是一片投递的全部上下文（站内与外部信封共用标题/正文/事件键）。
type sliceRef struct {
	slug    string
	leader  string
	level   string
	title   string
	body    string
	eventID string
	scope   map[string]any
	ev      *outlet.AlertEvent
}

// deliverSlice 投递一片：站内一条回执（channel=internal，收件人 leader
// 或默认接收组）+ 外部一条回执（channel=实际收尾出口或 skipped）。
func (s *Service) deliverSlice(ctx context.Context, ref sliceRef, now time.Time) []PushStatusEntry {
	entries := make([]PushStatusEntry, 0, 2)

	// 站内：直接落 messages（To=leader 账号；无 leader 的片不产生站内行）。
	station := PushStatusEntry{
		Slug: ref.slug, Leader: ref.leader, Channel: "internal", PushedAt: now,
		EventID: ref.eventID, OK: false, Error: "station sink not wired",
	}
	if ref.leader != "" || s.svcCtx.StationSink != nil {
		recipient := ref.leader
		if recipient == "" {
			recipient = outlet.DefaultStationRecipient
		}
		if sink := s.svcCtx.StationSink; sink != nil {
			err := sink.Create(ctx, outlet.StationNotice{
				To: recipient, Type: "report", Title: ref.title, Content: ref.body,
				Level: ref.level, Source: "incident_report",
				RefType: "report", RefID: ref.eventID, Scope: ref.scope,
			})
			if err != nil {
				station.Error = err.Error()
			} else {
				station.OK = true
				station.Error = ""
			}
		}
	}
	entries = append(entries, station)

	// 外部：出口链同步收尾回执。
	external := PushStatusEntry{
		Slug: ref.slug, Leader: ref.leader, Channel: "skipped", PushedAt: now,
		EventID: ref.eventID, OK: false, Error: "outlets not wired",
	}
	if manager := s.svcCtx.OutletManager; manager != nil && ref.ev != nil {
		outcome, err := manager.DispatchExternal(ctx, *ref.ev)
		external.Channel = outcome.Channel
		if err != nil {
			external.Error = err.Error()
		} else if outcome.Delivered {
			external.OK = true
			external.Error = ""
		} else {
			external.Error = "no external outlet accepted"
		}
	}
	entries = append(entries, external)
	return entries
}

// aggregateEvent 总聚合信封：标题带总量与环比，正文列 Top 类别摘要。
func (s *Service) aggregateEvent(base, periodType, periodKey string, payload *ReportPayload, now time.Time) outlet.AlertEvent {
	sum := payload.Summary
	title := fmt.Sprintf("%s %s：共 %s 起", periodLabel(periodType), periodKey, formatCount(sum.Metrics.Total.Value))
	body := fmt.Sprintf("事故 %d + bug %d；影响时长 %s；MTTR %s；复发率 %s。",
		sum.Incidents, sum.Bugs,
		formatMs(sum.Metrics.Duration.Value), formatMs(sum.Metrics.MTTR.Value), formatPct(sum.Metrics.Recurrence.Value))
	top := payload.Categories
	if len(top) > 3 {
		top = top[:3]
	}
	for _, c := range top {
		name := c.Name
		if name == "" {
			name = c.Slug
		}
		body += fmt.Sprintf("\n· %s：%d 起（%s）", name, c.Count, c.Level)
	}
	return outlet.AlertEvent{
		Kind:             outlet.KindIncidentReport,
		Severity:         levelToSeverity(payload.Level),
		EventID:          base + ":all",
		DedupKey:         base + ":all",
		Title:            title,
		Body:             body,
		OccurredAtUnixMs: now.UnixMilli(),
	}
}

// sliceEvent 类别分片信封：站内经 category-leader 前缀投 leader，外部带
// 同 target；scope 携类别 slug（messages 读时按类别受众过滤，批 4d）。
func (s *Service) sliceEvent(base, periodType, periodKey string, piece *ReportCategorySlice, now time.Time) outlet.AlertEvent {
	name := piece.Name
	if name == "" {
		name = piece.Slug
	}
	title := fmt.Sprintf("%s %s · %s：%d 起（%s）", periodLabel(periodType), periodKey, name, piece.Count, piece.Level)
	body := fmt.Sprintf("环比 %s；复发率 %.0f%%；critical 未解决 %d。",
		formatDelta(piece.Count, piece.PrevCount), piece.RecurrencePct, piece.UnresolvedCritical)
	return outlet.AlertEvent{
		Kind:             outlet.KindIncidentReport,
		Severity:         levelToSeverity(piece.Level),
		EventID:          base + ":" + piece.Slug,
		DedupKey:         base + ":" + piece.Slug,
		Title:            title,
		Body:             body,
		OccurredAtUnixMs: now.UnixMilli(),
		Target:           "category-leader:" + piece.Slug,
		Scope:            outlet.Scope{CategorySlug: piece.Slug},
	}
}

// eventIDBase 确定性事件主键前缀（§6：report:<PeriodType>:<PeriodStart>）。
func eventIDBase(periodType, periodKey string) string {
	return "report:" + periodType + ":" + periodKey
}

// ---- 存量报表列表 ----

// StoredReportItem 报表历史行（payload 摘要 + 分发回执；面板历史页与重推
// 按钮数据源）。
type StoredReportItem struct {
	ID          uint              `json:"id"`
	PeriodType  string            `json:"periodType"`
	PeriodStart string            `json:"periodStart"`
	ReportKind  string            `json:"reportKind"`
	GeneratedAt time.Time         `json:"generatedAt"`
	Level       string            `json:"level"`
	Total       *float64          `json:"total"`
	PushStatus  []PushStatusEntry `json:"pushStatus"`
}

// StoredReportListResponse 报表历史分页（GET /incident-reports/stored）。
type StoredReportListResponse struct {
	Items []StoredReportItem `json:"items"`
	Total int64              `json:"total"`
}

// ListStoredReports 报表历史分页（periodType 空 = 全档位）。
func (s *Service) ListStoredReports(ctx context.Context, periodType string, page, pageSize int) (*StoredReportListResponse, error) {
	if periodType != "" &&
		periodType != PeriodWeek && periodType != PeriodMonth &&
		periodType != PeriodQuarter && periodType != PeriodYear {
		return nil, errorx.NewBadRequest("无效的报表档位: " + periodType)
	}
	db := s.svcCtx.DB.WithContext(ctx).Model(&model.IncidentReport{})
	if periodType != "" {
		db = db.Where("period_type = ?", periodType)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var rows []model.IncidentReport
	if err := db.Order("period_start DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]StoredReportItem, 0, len(rows))
	for i := range rows {
		item := StoredReportItem{
			ID: rows[i].ID, PeriodType: rows[i].PeriodType, PeriodStart: rows[i].PeriodStart,
			ReportKind: rows[i].ReportKind, GeneratedAt: rows[i].GeneratedAt,
		}
		var payload ReportPayload
		if json.Unmarshal(rows[i].Payload, &payload) == nil && payload.Summary != nil {
			item.Level = payload.Level
			item.Total = payload.Summary.Metrics.Total.Value
		}
		if len(rows[i].PushStatus) > 0 {
			_ = json.Unmarshal(rows[i].PushStatus, &item.PushStatus)
		}
		items = append(items, item)
	}
	return &StoredReportListResponse{Items: items, Total: total}, nil
}

// ---- 小工具 ----

func sliceLevelRank(level string) int {
	switch level {
	case model.MessageLevelCritical:
		return 2
	case model.MessageLevelWarn:
		return 1
	default:
		return 0
	}
}

func levelToSeverity(level string) string {
	switch level {
	case model.MessageLevelCritical:
		return outlet.SeverityCritical
	case model.MessageLevelWarn:
		return outlet.SeverityWarning
	default:
		return outlet.SeverityInfo
	}
}

// periodLabel 档位中文展示名（通知标题用）。
func periodLabel(periodType string) string {
	switch periodType {
	case PeriodWeek:
		return "事故周报"
	case PeriodMonth:
		return "事故月报"
	case PeriodQuarter:
		return "事故季报"
	case PeriodYear:
		return "事故年报"
	default:
		return "事故报表"
	}
}

func formatCount(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f", *v)
}

func formatMs(v *float64) string {
	if v == nil {
		return "-"
	}
	d := time.Duration(*v) * time.Millisecond
	if d >= 24*time.Hour {
		return fmt.Sprintf("%.1fd", d.Hours()/24)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	if d >= time.Minute {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

func formatPct(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", *v*100)
}

// formatDelta 环比差（cur-prev 与方向箭头；基期 0 记 "+N"）。
func formatDelta(cur, prev int64) string {
	switch {
	case prev <= 0:
		return fmt.Sprintf("+%d", cur)
	case cur > prev:
		return fmt.Sprintf("+%d（+%d%%）", cur-prev, (cur-prev)*100/prev)
	case cur < prev:
		return fmt.Sprintf("-%d（-%d%%）", prev-cur, (prev-cur)*100/prev)
	default:
		return "持平"
	}
}
