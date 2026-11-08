package incident

// report.go 补测：bucketKey 全桶形态、趋势缺省窗口（week/month）、
// dataEpoch 变体（仅 bug / bug 早于事故）、占比与榜单同值并列的次序兜底、
// 开放事故拖尾时长、MTTR 基期空样本 missing、W53 同比缺失三档贯通。

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

func TestBucketKey_WeekMonthDay(t *testing.T) {
	wed := time.Date(2026, 11, 4, 15, 0, 0, 0, time.Local) // 周三
	sun := time.Date(2026, 11, 8, 15, 0, 0, 0, time.Local) // 周日

	// week 桶键 = 该 ISO 周周一；周日归前一个周一
	require.Equal(t, "2026-11-02", bucketKey(BucketWeek, wed))
	require.Equal(t, "2026-11-02", bucketKey(BucketWeek, sun))
	require.Equal(t, "2026-11", bucketKey(BucketMonth, wed))
	require.Equal(t, "2026-11-04", bucketKey(BucketDay, wed))
}

func TestReportTrend_DefaultWindowsAndBuckets(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	wed := time.Date(2026, 11, 4, 10, 0, 0, 0, time.Local)
	seedIncident(t, s, qa.ID, wed, wed.Add(2*time.Hour), "", "", model.IncidentRespOperator, "op-1")

	// 空 bucket 默认 day；今年数据对去年窗口 → 同比缺失
	resp, err := s.ReportTrend(ctx, "", nil, nil)
	require.NoError(t, err)
	require.Equal(t, BucketDay, resp.Bucket)
	require.True(t, resp.YoyMissing, "this-year seed leaves last-year window empty")
	require.Len(t, resp.Points, 1)

	// week 缺省窗口 = 近 12 周；month = 近 12 个月
	resp, err = s.ReportTrend(ctx, BucketWeek, nil, nil)
	require.NoError(t, err)
	require.Equal(t, BucketWeek, resp.Bucket)
	require.Equal(t, "2026-11-02", resp.Points[0].Bucket)
	require.GreaterOrEqual(t, len(resp.Points), 1)

	resp, err = s.ReportTrend(ctx, BucketMonth, nil, nil)
	require.NoError(t, err)
	require.Equal(t, BucketMonth, resp.Bucket)
	require.Equal(t, "2026-11", resp.Points[0].Bucket)
}

func TestReportTrend_TwoCategoriesSortedByID(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	client := mustCategory(t, s, model.IncidentCatClient)
	qa := mustCategory(t, s, model.IncidentCatQA)
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 11, 8, 0, 0, 0, 0, time.Local)
	seedIncident(t, s, qa.ID, from.Add(24*time.Hour), from.Add(26*time.Hour), "", "", model.IncidentRespOperator, "op-1")
	seedIncident(t, s, client.ID, from.Add(48*time.Hour), from.Add(50*time.Hour), "", "", model.IncidentRespOperator, "op-2")

	_, cats, err := s.trendPoints(ctx, BucketDay, from, to)
	require.NoError(t, err)
	require.Len(t, cats, 2)
	require.Less(t, cats[0].ID, cats[1].ID, "categories must sort by id ascending")
}

func TestReportSummary_DataEpochVariants(t *testing.T) {
	// 仅 bug：epoch 取 bug created_at（无事故分支）
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	seedBug(t, s, qa.ID, p.Start.Add(2*time.Hour))
	resp, err := s.ReportSummary(ctx, PeriodWeek, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Bugs)

	// bug 早于最早事故：epoch 取更早的 bug（bugMin 分支）
	s2 := setupService(t)
	seedBug(t, s2, qa.ID, time.Date(2026, 1, 5, 0, 0, 0, 0, time.Local))
	seedIncident(t, s2, qa.ID, time.Date(2026, 2, 5, 0, 0, 0, 0, time.Local),
		time.Date(2026, 2, 5, 2, 0, 0, 0, time.Local), "", "", model.IncidentRespOperator, "op-1")
	_, err = s2.ReportSummary(ctx, PeriodWeek, "")
	require.NoError(t, err)
}

func TestReportSummary_CategoryShareTieBreaksByID(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	client := mustCategory(t, s, model.IncidentCatClient)
	qa := mustCategory(t, s, model.IncidentCatQA)
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	at := p.Start.Add(2 * time.Hour)
	seedIncident(t, s, qa.ID, at, at.Add(time.Hour), "", "", model.IncidentRespOperator, "op-1")
	seedIncident(t, s, client.ID, at, at.Add(time.Hour), "", "", model.IncidentRespOperator, "op-2")

	resp, err := s.ReportSummary(ctx, PeriodWeek, "")
	require.NoError(t, err)
	require.Len(t, resp.Breakdown, 2)
	// 计数并列（各 1）→ 按 categoryID 升序兜底
	require.Less(t, resp.Breakdown[0].CategoryID, resp.Breakdown[1].CategoryID)
}

func TestReportLeaderboard_OpenTailAndMTTRMissingBase(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	// 查询封闭的上一周（对 now 稳定）：开放事故的拖尾按整期计，无挂钟边缘
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	curStart := p.PrevStart
	iy, iw := curStart.ISOWeek()
	curKey := fmt.Sprintf("%d-W%02d", iy, iw)
	baseStart := curStart.AddDate(0, 0, -7) // 基期 = 上上周

	// 查询周：op-1 已解决；op-2 未解决（封闭期拖尾 = 期末-发生）
	seedIncident(t, s, qa.ID, curStart.Add(time.Hour), curStart.Add(3*time.Hour),
		"", "", model.IncidentRespOperator, "op-1")
	seedIncident(t, s, qa.ID, curStart.Add(2*time.Hour), time.Time{},
		"", "", model.IncidentRespOperator, "op-2")
	// 基期：op-1 有一条未解决（基期无 resolved 样本 → MTTR 对照 missing）
	seedIncident(t, s, qa.ID, baseStart.Add(time.Hour), time.Time{},
		"", "", model.IncidentRespOperator, "op-1")

	resp, err := s.ReportLeaderboard(ctx, PeriodWeek, curKey, ViewResponsible, BoardDuration)
	require.NoError(t, err)
	byKey := map[string]LeaderboardRow{}
	for _, r := range resp.Rows {
		byKey[r.Key] = r
	}
	require.Greater(t, byKey["op-2"].DurationMS, int64(0), "open incident tail must count")

	resp, err = s.ReportLeaderboard(ctx, PeriodWeek, curKey, ViewResponsible, BoardMTTR)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Rows, "op-1 has resolved sample and must rank")
	for _, r := range resp.Rows {
		require.True(t, r.Prev.Missing, "base week op-1 has no resolved sample")
	}
}

func TestReportYoyMissing_W53AcrossTiers(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	client := mustCategory(t, s, model.IncidentCatClient)
	qa := mustCategory(t, s, model.IncidentCatQA)
	// 2026-W53 = 2026-12-28（周一）~2027-01-03；2025 无 W53 → 同比缺失
	w53 := time.Date(2026, 12, 29, 10, 0, 0, 0, time.Local)
	seedIncident(t, s, qa.ID, w53, w53.Add(time.Hour), "", "", model.IncidentRespOperator, "op-1")
	seedIncident(t, s, client.ID, w53, w53.Add(time.Hour), "", "", model.IncidentRespOperator, "op-2")

	lb, err := s.ReportLeaderboard(ctx, PeriodWeek, "2026-W53", ViewResponsible, BoardIncidents)
	require.NoError(t, err)
	require.Len(t, lb.Rows, 2)
	for _, r := range lb.Rows {
		require.NotNil(t, r.Yoy)
		require.True(t, r.Yoy.Missing, "W53 has no last-year counterpart")
	}
	// 同值并列 → 按 Key 兜底排序
	require.Less(t, lb.Rows[0].Key, lb.Rows[1].Key)

	rp, err := s.ReportResponsibility(ctx, PeriodWeek, "2026-W53")
	require.NoError(t, err)
	require.Len(t, rp.Categories, 2)
	for _, c := range rp.Categories {
		require.True(t, c.Yoy.Missing)
	}
	require.Less(t, rp.Categories[0].CategoryID, rp.Categories[1].CategoryID, "tie breaks by category id")

	require.Len(t, rp.Responsibles, 2)
	for _, r := range rp.Responsibles {
		require.True(t, r.Yoy.Missing)
	}
	require.Less(t, rp.Responsibles[0].Key, rp.Responsibles[1].Key, "tie breaks by key")
}

func TestReportResponsibility_BadPeriodKey(t *testing.T) {
	s := setupService(t)
	_, err := s.ReportResponsibility(context.Background(), PeriodWeek, "bogus")
	require.Error(t, err)
}
