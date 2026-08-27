package approvals

import (
	"testing"
	"time"
)

// TestOutsideWindowRobustAcrossDay 用一天内每个分钟作为 now 验证
// 「未来窗口」构造必然 outside（模拟任意 CI 运行时刻）。
func TestOutsideWindowRobustAcrossDay(t *testing.T) {
	svc := NewDelegationService(&configurableDelegationStore{}, nil, nil)
	base := time.Date(2026, 8, 27, 0, 0, 30, 0, time.UTC) // 秒取 30 覆盖半分
	for m := 0; m < 1440; m++ {
		now := base.Add(time.Duration(m) * time.Minute)
		got := svc.checkTimeRestriction(map[string]interface{}{
			"allowed_start": now.Add(time.Hour).Format("15:04"),
			"allowed_end":   now.Add(2 * time.Hour).Format("15:04"),
		}, now)
		if got {
			t.Fatalf("minute %d (%s): expected outside, got inside (window %s-%s)",
				m, now.Format("15:04:00"),
				now.Add(time.Hour).Format("15:04"), now.Add(2*time.Hour).Format("15:04"))
		}
	}
}
