package config

import "testing"

// TestExecutionLogConfigIsEnabledV9 covers the default-on semantics: an
// absent flag keeps the execution log enabled, explicit false disables it.
func TestExecutionLogConfigIsEnabledV9(t *testing.T) {
	cases := []struct {
		name string
		cfg  ExecutionLogConfig
		want bool
	}{
		{"default enabled", ExecutionLogConfig{}, true},
		{"explicit true", ExecutionLogConfig{Enabled: boolPtr(true)}, true},
		{"explicit false", ExecutionLogConfig{Enabled: boolPtr(false)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.IsEnabled(); got != tc.want {
				t.Fatalf("IsEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestExecutionLogConfigEffectiveRetentionDaysV9 covers the retention
// default (7 days) and explicit overrides, including 0 = keep forever.
func TestExecutionLogConfigEffectiveRetentionDaysV9(t *testing.T) {
	cases := []struct {
		name string
		cfg  ExecutionLogConfig
		want int
	}{
		{"default 7 days", ExecutionLogConfig{}, 7},
		{"explicit override", ExecutionLogConfig{RetentionDays: intPtr(30)}, 30},
		{"explicit zero keeps forever", ExecutionLogConfig{RetentionDays: intPtr(0)}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.EffectiveRetentionDays(); got != tc.want {
				t.Fatalf("EffectiveRetentionDays() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestTaskLogConfigEffectiveRetentionDaysV9 covers the task-log retention
// default (7 days) and explicit overrides, including 0 = keep forever.
func TestTaskLogConfigEffectiveRetentionDaysV9(t *testing.T) {
	cases := []struct {
		name string
		cfg  TaskLogConfig
		want int
	}{
		{"default 7 days", TaskLogConfig{}, 7},
		{"explicit override", TaskLogConfig{RetentionDays: intPtr(14)}, 14},
		{"explicit zero keeps forever", TaskLogConfig{RetentionDays: intPtr(0)}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.EffectiveRetentionDays(); got != tc.want {
				t.Fatalf("EffectiveRetentionDays() = %d, want %d", got, tc.want)
			}
		})
	}
}

func boolPtr(v bool) *bool { return &v }

func intPtr(v int) *int { return &v }
