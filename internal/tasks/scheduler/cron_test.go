package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCron_Valid(t *testing.T) {
	cases := []struct {
		expr  string
		match time.Time
		hit   bool
	}{
		{"* * * * *", time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC), true},
		{"0 0 * * *", time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC), true},
		{"0 0 * * *", time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC), false},
		{"*/15 * * * *", time.Date(2026, 8, 26, 10, 45, 0, 0, time.UTC), true},
		{"*/15 * * * *", time.Date(2026, 8, 26, 10, 43, 0, 0, time.UTC), false},
		{"30 10 * * 3", time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC), true}, // 周三
		{"0 9-17 * * *", time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC), true},
		{"0 9-17 * * *", time.Date(2026, 8, 26, 18, 0, 0, 0, time.UTC), false},
		{"1,15,30 2 * * *", time.Date(2026, 8, 26, 2, 15, 0, 0, time.UTC), true},
		{"5/10 * * * *", time.Date(2026, 8, 26, 2, 15, 0, 0, time.UTC), true},  // 5,15,25,...
		{"5/10 * * * *", time.Date(2026, 8, 26, 2, 13, 0, 0, time.UTC), false}, // 13 不在 5+10k
		{"0 0 1 * *", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), true},
		{"0 0 29 2 *", time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC), true}, // 闰年
	}
	for _, tc := range cases {
		spec, err := ParseCron(tc.expr)
		require.NoError(t, err, tc.expr)
		assert.Equal(t, tc.hit, spec.Matches(tc.match), "%s vs %s", tc.expr, tc.match)
	}
}

func TestParseCron_Invalid(t *testing.T) {
	for _, expr := range []string{
		"", "* * * *", "* * * * * *", "60 * * * *", "* 24 * * *",
		"* * 0 * *", "* * * 13 *", "* * * * 7", "a * * * *", "*/0 * * * *",
		"5-1 * * * *",
	} {
		_, err := ParseCron(expr)
		assert.Error(t, err, expr)
	}
}

func TestCronNext(t *testing.T) {
	spec, err := ParseCron("30 2 * * *") // 每天 02:30
	require.NoError(t, err)

	base := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	next := spec.Next(base)
	assert.Equal(t, time.Date(2026, 8, 27, 2, 30, 0, 0, time.UTC), next)

	// 严格大于：当前即命中时取下一个。
	at := time.Date(2026, 8, 27, 2, 30, 0, 0, time.UTC)
	next = spec.Next(at)
	assert.Equal(t, time.Date(2026, 8, 28, 2, 30, 0, 0, time.UTC), next)

	// 月末跨越。
	spec2, _ := ParseCron("0 0 1 * *")
	next2 := spec2.Next(time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), next2)
}

func TestDayDOWOrSemantics(t *testing.T) {
	// 2026-08-26 是周三（3）。日=25 或 周=3 都应命中 OR 语义。
	spec, err := ParseCron("0 0 25 * 3")
	require.NoError(t, err)
	assert.True(t, spec.Matches(time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)))
}

func TestSlotAlignment(t *testing.T) {
	in := time.Date(2026, 8, 26, 10, 30, 45, 999, time.UTC)
	assert.Equal(t, time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC), Slot(in))
}
