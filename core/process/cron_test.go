package process

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCronMatches(t *testing.T) {
	cases := []struct {
		expression string
		now        time.Time
		expected   bool
	}{
		{"* * * * *", time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC), true},
		{"0 0 * * *", time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC), true},
		{"0 0 * * *", time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC), false},
		{"*/15 * * * *", time.Date(2026, 8, 26, 10, 45, 0, 0, time.UTC), true},
		{"*/15 * * * *", time.Date(2026, 8, 26, 10, 43, 0, 0, time.UTC), false},
		{"30 10 * * 3", time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC), true},
		{"0 9-17 * * *", time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC), true},
		{"0 9-17 * * *", time.Date(2026, 8, 26, 18, 0, 0, 0, time.UTC), false},
		{"1,15,30 2 * * *", time.Date(2026, 8, 26, 2, 15, 0, 0, time.UTC), true},
		{"5/10 * * * *", time.Date(2026, 8, 26, 2, 15, 0, 0, time.UTC), true},
		{"5/10 * * * *", time.Date(2026, 8, 26, 2, 13, 0, 0, time.UTC), false},
		{"0 0 29 2 *", time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC), true},
	}
	for _, testCase := range cases {
		spec, err := ParseCron(testCase.expression)
		require.NoError(t, err, testCase.expression)
		assert.Equal(t, testCase.expected, spec.Matches(testCase.now), testCase.expression)
	}
}

func TestParseCronRejectsInvalidExpressions(t *testing.T) {
	for _, expression := range []string{
		"", "* * * *", "* * * * * *", "60 * * * *", "* 24 * * *",
		"* * 0 * *", "* * * 13 *", "* * * * 7", "a * * * *", "*/0 * * * *",
		"5-1 * * * *",
	} {
		_, err := ParseCron(expression)
		assert.Error(t, err, expression)
	}
}

func TestCronNext(t *testing.T) {
	spec, err := ParseCron("30 2 * * *")
	require.NoError(t, err)

	next := spec.Next(time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC))
	assert.Equal(t, time.Date(2026, 8, 27, 2, 30, 0, 0, time.UTC), next)
	next = spec.Next(time.Date(2026, 8, 27, 2, 30, 0, 0, time.UTC))
	assert.Equal(t, time.Date(2026, 8, 28, 2, 30, 0, 0, time.UTC), next)

	monthly, err := ParseCron("0 0 1 * *")
	require.NoError(t, err)
	next = monthly.Next(time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), next)
}

func TestDayWeekdayOrSemantics(t *testing.T) {
	spec, err := ParseCron("0 0 25 * 3")
	require.NoError(t, err)
	assert.True(t, spec.Matches(time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)))
}

func TestScheduleCompilationAndNext(t *testing.T) {
	entries := CompileSchedule(map[string][]ScheduleAction{
		"app": {
			{Action: "start", Cron: "* * * * *"},
			{Action: "stop", Cron: "bad cron"},
			{Action: "restart", Cron: "30 2 * * *"},
		},
	})
	require.Len(t, entries["app"], 2)

	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	next, action := NextSchedule(entries["app"], now)
	assert.Equal(t, "start", action)
	assert.Equal(t, time.Date(2026, 8, 26, 10, 1, 0, 0, time.UTC), next)
}

func TestSlotAlignsToMinute(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 30, 45, 999, time.UTC)
	assert.Equal(t, time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC), Slot(now))
}
