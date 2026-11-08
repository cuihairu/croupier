package incident

// resolvePeriod 四档位解析：当前周期（空 key）、显式 key、坏 key、以及
// W53 同比缺失分支。此前月/季显式键与年/周空键、YoyMissing 均无用例。

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolvePeriod_InvalidType(t *testing.T) {
	_, err := resolvePeriod("day", "2026-01-01", time.Now())
	require.Error(t, err)
}

func TestResolveWeekPeriod_ExplicitKeyAndBounds(t *testing.T) {
	now := time.Date(2026, time.November, 8, 0, 0, 0, 0, time.Local)

	p, err := resolvePeriod(PeriodWeek, "2026-W41", now)
	require.NoError(t, err)
	require.Equal(t, "2026-W41", p.Key)
	require.Equal(t, 7, int(p.End.Sub(p.Start).Hours()/24))
	require.Equal(t, p.Start.AddDate(0, 0, -7), p.PrevStart)
	require.Equal(t, p.Start, p.PrevEnd)
	require.False(t, p.YoyMissing)
	require.False(t, p.YoyStart.IsZero())

	// 2026 是 53 周年；2025 只有 52 周 → 去年同周号缺失 → YoyMissing。
	p, err = resolvePeriod(PeriodWeek, "2026-W53", now)
	require.NoError(t, err)
	require.Equal(t, "2026-W53", p.Key)
	require.True(t, p.YoyMissing)
	require.True(t, p.YoyStart.IsZero())

	// 坏形态与越界周号。
	for _, key := range []string{"2026/13", "W41", "2026-W0", "2026-W54"} {
		_, err = resolvePeriod(PeriodWeek, key, now)
		require.Error(t, err, "key %q must be rejected", key)
	}
}

func TestResolveMonthPeriod_CurrentExplicitAndBad(t *testing.T) {
	now := time.Date(2026, time.November, 8, 0, 0, 0, 0, time.Local)

	p, err := resolvePeriod(PeriodMonth, "", now)
	require.NoError(t, err)
	require.Equal(t, "2026-11", p.Key)
	require.Equal(t, time.Date(2026, time.November, 1, 0, 0, 0, 0, time.Local), p.Start)

	p, err = resolvePeriod(PeriodMonth, "2026-03", now)
	require.NoError(t, err)
	require.Equal(t, "2026-03", p.Key)
	require.Equal(t, time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), p.Start)
	require.Equal(t, p.Start.AddDate(0, -1, 0), p.PrevStart)
	require.Equal(t, time.Date(2025, time.March, 1, 0, 0, 0, 0, time.Local), p.YoyStart)

	for _, key := range []string{"2026-3", "2026-13", "2026Q1"} {
		_, err = resolvePeriod(PeriodMonth, key, now)
		require.Error(t, err, "key %q must be rejected", key)
	}
}

func TestResolveQuarterPeriod_CurrentExplicitAndBad(t *testing.T) {
	now := time.Date(2026, time.November, 8, 0, 0, 0, 0, time.Local)

	p, err := resolvePeriod(PeriodQuarter, "", now)
	require.NoError(t, err)
	require.Equal(t, "2026-Q4", p.Key)
	require.Equal(t, time.Date(2026, time.October, 1, 0, 0, 0, 0, time.Local), p.Start)

	p, err = resolvePeriod(PeriodQuarter, "2026-Q2", now)
	require.NoError(t, err)
	require.Equal(t, "2026-Q2", p.Key)
	require.Equal(t, time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), p.Start)
	require.Equal(t, p.End.AddDate(0, -3, 0), p.PrevEnd)
	require.Equal(t, time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local), p.YoyStart)

	_, err = resolvePeriod(PeriodQuarter, "2026-Q5", now)
	require.Error(t, err)
	_, err = resolvePeriod(PeriodQuarter, "2026-2", now)
	require.Error(t, err)
}

func TestResolveYearPeriod_CurrentExplicitAndBad(t *testing.T) {
	now := time.Date(2026, time.November, 8, 0, 0, 0, 0, time.Local)

	p, err := resolvePeriod(PeriodYear, "", now)
	require.NoError(t, err)
	require.Equal(t, "2026", p.Key)
	require.True(t, p.YoyMissing, "year tier has no yoy column")

	p, err = resolvePeriod(PeriodYear, "2024", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2024, time.January, 1, 0, 0, 0, 0, time.Local), p.Start)
	require.Equal(t, time.Date(2023, time.January, 1, 0, 0, 0, 0, time.Local), p.PrevStart)

	_, err = resolvePeriod(PeriodYear, "26", now)
	require.Error(t, err)
}
