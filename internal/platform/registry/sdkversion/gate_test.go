package sdkversion

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		name           string
		current, high  string
		want           Verdict
		wantHigherThan string // Higher(current, high) expectation when set
	}{
		// 高于/等于高水位：放行
		{"equal", "0.1.4", "0.1.4", VerdictAllow, ""},
		{"same major minor equal diff patch", "0.1.5", "0.1.4", VerdictAllow, "yes"},
		{"higher minor", "0.2.0", "0.1.9", VerdictAllow, "yes"},
		{"higher major", "1.0.0", "0.9.9", VerdictAllow, "yes"},
		{"two segment equal", "0.1", "0.1.0", VerdictAllow, ""},

		// 低于但差距小于门槛（同 major，minor 差 1）：放行 + 警告
		{"behind one minor", "0.1.4", "0.2.0", VerdictAllowWithWarning, "no"},
		{"behind one minor patch lower", "0.2.1", "0.3.9", VerdictAllowWithWarning, "no"},

		// 低 2 个 minor：拒绝
		{"behind two minors", "0.1.4", "0.3.0", VerdictReject, "no"},
		{"behind two minors same patch", "0.1.0", "0.3.0", VerdictReject, "no"},
		{"behind many minors", "0.1.0", "0.9.0", VerdictReject, "no"},

		// 低 1 个 major：拒绝（即使 minor 只差 0）
		{"behind one major", "0.9.9", "1.0.0", VerdictReject, "no"},
		{"behind one major same minor", "0.3.0", "1.3.0", VerdictReject, "no"},
		{"behind two majors", "0.1.0", "2.0.0", VerdictReject, "no"},

		// 解析失败：一律放行，不当高水位
		{"current unknown", "unknown", "0.3.0", VerdictAllow, "no"},
		{"current empty", "", "0.3.0", VerdictAllow, "no"},
		{"high unknown", "0.1.0", "unknown", VerdictAllow, "no"},
		{"high empty", "0.1.0", "", VerdictAllow, "no"},
		{"both unknown", "unknown", "unknown", VerdictAllow, "no"},
		{"current garbage", "v0.1.0", "0.3.0", VerdictAllow, "no"},
		{"current negative", "-1.2", "0.3.0", VerdictAllow, "no"},

		// pre-release/build 后缀按数字前缀参与
		{"prerelease current", "0.3.0-rc1", "0.3.0", VerdictAllow, ""},
		{"prerelease high", "0.1.0", "0.3.0-rc1", VerdictReject, "no"},

		// 一段式版本
		{"single segment equal", "1", "1.0", VerdictAllow, ""},
		{"single segment behind", "0", "1.0", VerdictReject, "no"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compare(tc.current, tc.high); got != tc.want {
				t.Errorf("Compare(%q, %q) = %v, want %v", tc.current, tc.high, got, tc.want)
			}
			if tc.wantHigherThan != "" {
				want := tc.wantHigherThan == "yes"
				if got := Higher(tc.current, tc.high); got != want {
					t.Errorf("Higher(%q, %q) = %v, want %v", tc.current, tc.high, got, want)
				}
			}
		})
	}
}

func TestHigherUnparseableNeverWins(t *testing.T) {
	if Higher("unknown", "0.1.0") {
		t.Error("unknown version must never compare higher")
	}
	if Higher("9.9.9", "unknown") {
		t.Error("comparison against unknown must never report higher")
	}
}

func TestParseable(t *testing.T) {
	for _, s := range []string{"0.1.4", "1.2", "3", "0.3.0-rc1"} {
		if !Parseable(s) {
			t.Errorf("Parseable(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "unknown", "v1.2", "1.2.3.4", "1..2", "1.2.x"} {
		if Parseable(s) {
			t.Errorf("Parseable(%q) = true, want false", s)
		}
	}
}
