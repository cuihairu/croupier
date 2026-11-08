package incident

// report_generator.go 补测：调度 payload 坏 JSON、重推缺 summary、
// 历史 payload 损坏/无同类分片时不判恶化、站内 sink 失败与外部出口
// 全不接的回执、聚合信封 Top3 截断，及 periodLabel/format* 纯函数表。

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"github.com/stretchr/testify/require"
)

// failingSink 站内落库面恒失败（回执应记错误且 OK=false）。
type failingSink struct{}

func (failingSink) Create(context.Context, outlet.StationNotice) error {
	return errors.New("station down")
}

// noAcceptOutlet 外部出口接了但不收（Delivered=false 无错 → 记 no accept）。
type noAcceptOutlet struct{}

func (noAcceptOutlet) Name() string { return "noop" }

func (noAcceptOutlet) Deliver(context.Context, outlet.AlertEvent) (outlet.DeliveryOutcome, error) {
	return outlet.DeliveryOutcome{Delivered: false, Channel: "noop"}, nil
}

func TestRun_NilAndBadPayload(t *testing.T) {
	h := setupGenerator(t)
	require.Error(t, h.svc.Run(context.Background(), nil))

	err := h.svc.Run(context.Background(), &model.TaskSchedule{Name: "weekly", Payload: model.JSON("{bad")})
	require.ErrorContains(t, err, "parse payload")

	err = h.svc.Run(context.Background(), &model.TaskSchedule{Name: "weekly", Payload: model.JSON(`{}`)})
	require.ErrorContains(t, err, "missing period")
}

func TestRepushReport_MissingSummaryRejected(t *testing.T) {
	h := setupGenerator(t)
	row := &model.IncidentReport{
		PeriodType: PeriodWeek, PeriodStart: "2026-W40",
		ReportKind: model.ReportKindSummary, Payload: model.JSON(`{"level":"info"}`),
		GeneratedAt: time.Now(),
	}
	require.NoError(t, h.db.Create(row).Error)

	_, err := h.svc.RepushReport(context.Background(), row.ID)
	require.ErrorContains(t, err, "缺少 summary")
}

func TestGenerateReport_HistoryEdgeCasesDoNotEscalate(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	qa := mustCategory(t, h.svc, model.IncidentCatQA)
	// 锚定当前周起点：避免挂钟翻周把种子落到窗外
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	seedIncident(t, h.svc, qa.ID, p.Start.Add(2*time.Hour), p.Start.Add(3*time.Hour),
		"", "", model.IncidentRespOperator, "op-1")

	// 历史两期 payload 损坏 → 按无历史处理，不判连续恶化
	for _, key := range []string{"2026-W40", "2026-W41"} {
		require.NoError(t, h.db.Create(&model.IncidentReport{
			PeriodType: PeriodWeek, PeriodStart: key,
			ReportKind: model.ReportKindSummary, Payload: model.JSON("{bad"),
			GeneratedAt: time.Now(),
		}).Error)
	}
	row, err := h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err)
	var payload ReportPayload
	require.NoError(t, json.Unmarshal(row.Payload, &payload))
	require.NotEmpty(t, payload.Categories, "qa piece must exist")
	for _, c := range payload.Categories {
		require.Equal(t, model.MessageLevelInfo, c.Level, "corrupt history must not escalate")
	}

	// 历史存在但无同类 slug 分片 → 同样不判恶化
	h2 := setupGenerator(t)
	seedIncident(t, h2.svc, qa.ID, p.Start.Add(2*time.Hour), p.Start.Add(3*time.Hour),
		"", "", model.IncidentRespOperator, "op-1")
	for _, key := range []string{"2026-W40", "2026-W41"} {
		other, _ := json.Marshal(ReportPayload{
			Summary:    &ReportSummaryResponse{Period: PeriodWeek, PeriodKey: key},
			Categories: []ReportCategorySlice{{Slug: "other-slug", Count: 9}, {Slug: "", Count: 3}},
		})
		require.NoError(t, h2.db.Create(&model.IncidentReport{
			PeriodType: PeriodWeek, PeriodStart: key,
			ReportKind: model.ReportKindSummary, Payload: model.JSON(other),
			GeneratedAt: time.Now(),
		}).Error)
	}
	row, err = h2.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(row.Payload, &payload))
	require.NotEmpty(t, payload.Categories, "qa piece must exist")
	for _, c := range payload.Categories {
		require.Equal(t, model.MessageLevelInfo, c.Level, "history without matching slug must not escalate")
	}
}

func TestGenerateReport_StationSinkFailureAndNoAcceptOutlet(t *testing.T) {
	h := setupGenerator(t)
	qa := mustCategory(t, h.svc, model.IncidentCatQA)
	leader := "op-lead"
	h.setLeader(t, model.IncidentCatQA, leader)
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	seedIncident(t, h.svc, qa.ID, p.Start.Add(2*time.Hour), p.Start.Add(3*time.Hour),
		"", "", model.IncidentRespOperator, leader)

	// 站内恒失败：internal 回执 OK=false 且 Error 记原文
	h.svc.svcCtx.StationSink = failingSink{}
	h.svc.svcCtx.OutletManager = outlet.NewManager()
	h.svc.svcCtx.OutletManager.Register(noAcceptOutlet{})
	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.NoError(t, err)
	var entries []PushStatusEntry
	require.NoError(t, json.Unmarshal(row.PushStatus, &entries))
	require.NotEmpty(t, entries)
	sawStationFail, sawNoAccept := false, false
	for _, e := range entries {
		if e.Channel == "internal" {
			require.Equal(t, "station down", e.Error)
			require.False(t, e.OK)
			sawStationFail = true
		}
		if e.Channel == "noop" {
			require.False(t, e.OK)
			require.Contains(t, e.Error, "no external outlet accepted")
			sawNoAccept = true
		}
	}
	require.True(t, sawStationFail, "must have internal entries")
	require.True(t, sawNoAccept, "must have external entries")
}

func TestGenerateReport_AggregateBodyClipsTopThree(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	for i, slug := range []string{model.IncidentCatQA, model.IncidentCatClient, model.IncidentCatFrontend, model.IncidentCatOps} {
		h.setLeader(t, slug, "lead-"+slug)
		seedIncident(t, h.svc, mustCategory(t, h.svc, slug).ID, p.Start.Add(time.Duration(i+1)*time.Hour), p.Start.Add(time.Duration(i+1)*time.Hour+30*time.Minute),
			"", "", model.IncidentRespOperator, "lead-"+slug)
	}
	_, err = h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err)

	snap := h.capture.snapshot()
	var agg *outlet.AlertEvent
	for i := range snap {
		if strings.HasSuffix(snap[i].EventID, ":all") {
			agg = &snap[i]
			break
		}
	}
	require.NotNil(t, agg, "aggregate envelope must be dispatched")
	require.Equal(t, 3, strings.Count(agg.Body, "\n· "), "body must clip to top 3 categories")
}

func TestPeriodLabelAndFormatters(t *testing.T) {
	require.Equal(t, "事故周报", periodLabel(PeriodWeek))
	require.Equal(t, "事故月报", periodLabel(PeriodMonth))
	require.Equal(t, "事故季报", periodLabel(PeriodQuarter))
	require.Equal(t, "事故年报", periodLabel(PeriodYear))
	require.Equal(t, "事故报表", periodLabel("bogus"))

	ten := 10.0
	require.Equal(t, "-", formatMs(nil))
	require.Equal(t, "-", formatPct(nil))
	require.Equal(t, "10", formatCount(&ten))
	require.Equal(t, "-", formatCount(nil))

	day := float64((24 * time.Hour).Milliseconds())
	hour := float64(90 * time.Minute.Milliseconds())
	minute := float64(5 * time.Minute.Milliseconds())
	second := float64(30 * time.Second.Milliseconds())
	require.Contains(t, formatMs(&day), "d")
	require.Contains(t, formatMs(&hour), "h")
	require.Contains(t, formatMs(&minute), "m")
	require.Contains(t, formatMs(&second), "s")

	require.Equal(t, "+5", formatDelta(5, 0))
	require.Equal(t, "持平", formatDelta(3, 3))
	require.Equal(t, "+3（+60%）", formatDelta(8, 5))
	require.Equal(t, "-2（-40%）", formatDelta(3, 5))

	require.Equal(t, 2, sliceLevelRank(model.MessageLevelCritical))
	require.Equal(t, 1, sliceLevelRank(model.MessageLevelWarn))
	require.Equal(t, 0, sliceLevelRank(model.MessageLevelInfo))

	require.Equal(t, outlet.SeverityCritical, levelToSeverity(model.MessageLevelCritical))
	require.Equal(t, outlet.SeverityWarning, levelToSeverity(model.MessageLevelWarn))
	require.Equal(t, outlet.SeverityInfo, levelToSeverity(model.MessageLevelInfo))
}
