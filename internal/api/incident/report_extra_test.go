package incident

// Extra coverage for report.go: all bucketKey bucket shapes, trend default
// windows (week/month), dataEpoch variants (bug only / bug predating
// incidents), tie-break ordering for category share and leaderboards, open
// incident tail duration, MTTR missing with an empty base sample, and W53 YoY
// missing across all three report shapes.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

func TestBucketKey_WeekMonthDay(t *testing.T) {
	wed := time.Date(2026, 11, 4, 15, 0, 0, 0, time.Local) // Wednesday
	sun := time.Date(2026, 11, 8, 15, 0, 0, 0, time.Local) // Sunday

	// week bucket key = the Monday of that ISO week; a Sunday maps to the
	// preceding Monday
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

	// Empty bucket defaults to day; this-year data against the last-year
	// window -> YoY missing
	resp, err := s.ReportTrend(ctx, "", nil, nil)
	require.NoError(t, err)
	require.Equal(t, BucketDay, resp.Bucket)
	require.True(t, resp.YoyMissing, "this-year seed leaves last-year window empty")
	require.Len(t, resp.Points, 1)

	// week default window = the last 12 weeks; month = the last 12 months
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
	// Bug only: epoch takes the bug created_at (no-incident branch)
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	seedBug(t, s, qa.ID, p.Start.Add(2*time.Hour))
	resp, err := s.ReportSummary(ctx, PeriodWeek, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Bugs)

	// Bug earlier than the earliest incident: epoch takes the earlier bug
	// (bugMin branch)
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
	// Counts tied (1 each) -> tie-breaks by categoryID ascending
	require.Less(t, resp.Breakdown[0].CategoryID, resp.Breakdown[1].CategoryID)
}

func TestReportLeaderboard_OpenTailAndMTTRMissingBase(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	// Query a closed previous week (stable relative to now): an open incident's
	// tail counts for the whole period, no wall-clock edge
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	curStart := p.PrevStart
	iy, iw := curStart.ISOWeek()
	curKey := fmt.Sprintf("%d-W%02d", iy, iw)
	baseStart := curStart.AddDate(0, 0, -7) // base period = the week before last

	// Queried week: op-1 resolved; op-2 unresolved (closed-period tail = end
	// minus detection)
	seedIncident(t, s, qa.ID, curStart.Add(time.Hour), curStart.Add(3*time.Hour),
		"", "", model.IncidentRespOperator, "op-1")
	seedIncident(t, s, qa.ID, curStart.Add(2*time.Hour), time.Time{},
		"", "", model.IncidentRespOperator, "op-2")
	// Base period: op-1 has one unresolved incident (no resolved sample in the
	// base period -> MTTR comparison is missing)
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
	// 2026-W53 = 2026-12-28 (Monday) through 2027-01-03; 2025 has no W53 ->
	// YoY missing
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
	// Equal values tie -> tie-breaks by Key
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
