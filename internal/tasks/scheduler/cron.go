// Package scheduler 实现定时任务（cron 调度）。
//
// 设计要点：
//   - 五字段 cron（分 时 日 月 周），标准库自实现，零外部依赖
//   - 调度循环每 30s 扫描 task_schedules.next_triggered_at <= now 的行，
//     逐条触发；触发槽（分钟对齐）写 run log 做幂等窗口
//   - 触发即派发异步 TaskRun（复用 dispatch 链路），不等执行结果；
//     执行失败在下次触发前回查上一次 TaskRun 状态判定"连续失败"
//   - 连续失败达到 maxFailedRuns 进 dead_letter（暂停触发，等人工处理）
//   - 单实例部署模型（与当前 Server 一致）；多实例部署时靠触发槽
//     唯一索引兜底防止重复触发（见 TaskScheduleRunLog 唯一索引）
package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronSpec 是解析后的五字段 cron 表达式。
type CronSpec struct {
	Minute  uint64 // bit i 置位表示第 i 分钟命中
	Hour    uint64
	Day     uint64
	Month   uint64
	Weekday uint64 // 0=Sunday
}

// ParseCron 解析五字段 cron 表达式（分 时 日 月 周）。
// 支持 * 、数字、区间 a-b、步进 */n 与 a-b/n、逗号列表。
func ParseCron(expr string) (*CronSpec, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron 表达式需要 5 个字段（分 时 日 月 周），got %d", len(fields))
	}
	bounds := []struct{ min, max uint }{
		{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6},
	}
	spec := &CronSpec{}
	targets := []*uint64{&spec.Minute, &spec.Hour, &spec.Day, &spec.Month, &spec.Weekday}
	for i, f := range fields {
		v, err := parseField(f, bounds[i].min, bounds[i].max)
		if err != nil {
			return nil, fmt.Errorf("字段 %d（%s）: %w", i+1, f, err)
		}
		*targets[i] = v
	}
	return spec, nil
}

func parseField(f string, min, max uint) (uint64, error) {
	var bits uint64
	for _, part := range strings.Split(f, ",") {
		b, err := parsePart(part, min, max)
		if err != nil {
			return 0, err
		}
		bits |= b
	}
	return bits, nil
}

func parsePart(part string, min, max uint) (uint64, error) {
	rangePart, stepPart := part, "1"
	if idx := strings.Index(part, "/"); idx >= 0 {
		rangePart, stepPart = part[:idx], part[idx+1:]
	}
	step, err := strconv.ParseUint(stepPart, 10, 32)
	if err != nil || step == 0 {
		return 0, fmt.Errorf("非法步进 %q", stepPart)
	}

	lo, hi := min, max
	switch {
	case rangePart == "*" || rangePart == "":
		// 全区间
	case strings.Contains(rangePart, "-"):
		ab := strings.SplitN(rangePart, "-", 2)
		l, err1 := strconv.ParseUint(ab[0], 10, 32)
		h, err2 := strconv.ParseUint(ab[1], 10, 32)
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("非法区间 %q", rangePart)
		}
		lo, hi = uint(l), uint(h)
	default:
		v, err := strconv.ParseUint(rangePart, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("非法数值 %q", rangePart)
		}
		lo, hi = uint(v), uint(v)
		// "5/2" 语义：从 5 到 max 步进 2。
		if strings.Contains(part, "/") {
			hi = max
		}
	}
	if lo < min || hi > max || lo > hi {
		return 0, fmt.Errorf("数值超出范围 [%d,%d]", min, max)
	}
	var bits uint64
	for i := lo; i <= hi; i += uint(step) {
		bits |= 1 << i
	}
	return bits, nil
}

// Matches reports whether t hits the spec.
func (s *CronSpec) Matches(t time.Time) bool {
	return s.Minute&(1<<uint(t.Minute())) != 0 &&
		s.Hour&(1<<uint(t.Hour())) != 0 &&
		s.Month&(1<<uint(int(t.Month()))) != 0 &&
		s.dayMatches(t)
}

// dayMatches 实现标准 cron 的"日与周 OR"语义：
// 两个都为 * 时都命中；任一非 * 时按 OR 命中。
func (s *CronSpec) dayMatches(t time.Time) bool {
	dayHit := s.Day&(1<<uint(t.Day())) != 0
	dowHit := s.Weekday&(1<<uint(int(t.Weekday()))) != 0
	// day 字段取值 1..31：`*` 置位 bit1..31，即 (1<<32)-2。
	// 历史写成 (1<<32)-1（含 bit0），dayStar 恒 false，"日*周N"
	// 表达式会退化为每天触发。
	dayStar := s.Day == (1<<32)-2
	dowStar := s.Weekday == 127 // 0-6 全置
	switch {
	case dayStar && dowStar:
		return true
	case dayStar:
		return dowHit
	case dowStar:
		return dayHit
	default:
		return dayHit || dowHit
	}
}

// Next returns the next matching time strictly after t.
func (s *CronSpec) Next(t time.Time) time.Time {
	// 分钟对齐后逐分钟前进，最多扫描 5 年（闰年/月末组合上界）。
	cur := t.Truncate(time.Minute).Add(time.Minute)
	limit := t.AddDate(5, 0, 0)
	for cur.Before(limit) {
		if s.Matches(cur) {
			return cur
		}
		cur = cur.Add(time.Minute)
	}
	return time.Time{}
}

// Slot 把触发时间对齐到分钟（幂等窗口键）。
func Slot(t time.Time) time.Time { return t.Truncate(time.Minute) }
