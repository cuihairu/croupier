package spec

import "testing"

// NormalizeExecutionState 归一存量行：空值与未知值 → bound，仅 unbound
// （容忍首尾空白）保持 unbound。
func TestNormalizeExecutionState(t *testing.T) {
	cases := []struct {
		in   string
		want ExecutionState
	}{
		{"", ExecutionStateBound},
		{"bound", ExecutionStateBound},
		{"Bound", ExecutionStateBound},
		{"unbound", ExecutionStateUnbound},
		{"  unbound  ", ExecutionStateUnbound},
		{"whatever", ExecutionStateBound},
	}
	for _, tc := range cases {
		if got := NormalizeExecutionState(tc.in); got != tc.want {
			t.Errorf("NormalizeExecutionState(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
