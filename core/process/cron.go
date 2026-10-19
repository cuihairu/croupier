// Package process provides game-agnostic managed process lifecycle helpers.
package process

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CronSpec struct {
	Minute  uint64
	Hour    uint64
	Day     uint64
	Month   uint64
	Weekday uint64
}

type ScheduleAction struct {
	Action string
	Cron   string
}

type ScheduleEntry struct {
	Action string
	Cron   string
	Spec   *CronSpec
}

func ParseCron(expr string) (*CronSpec, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron requires 5 fields (minute hour day month weekday), got %d", len(fields))
	}
	spec := &CronSpec{}
	targets := []*uint64{&spec.Minute, &spec.Hour, &spec.Day, &spec.Month, &spec.Weekday}
	bounds := []struct{ min, max uint }{
		{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6},
	}
	for fieldIndex, field := range fields {
		bits, err := parseCronField(field, bounds[fieldIndex].min, bounds[fieldIndex].max)
		if err != nil {
			return nil, fmt.Errorf("field %d: %w", fieldIndex+1, err)
		}
		*targets[fieldIndex] = bits
	}
	return spec, nil
}

func Slot(now time.Time) time.Time {
	return now.Truncate(time.Minute)
}

func parseCronField(field string, min, max uint) (uint64, error) {
	var bits uint64
	for _, part := range strings.Split(field, ",") {
		partBits, err := parseCronPart(part, min, max)
		if err != nil {
			return 0, err
		}
		bits |= partBits
	}
	return bits, nil
}

func parseCronPart(part string, min, max uint) (uint64, error) {
	rangePart, stepPart := part, "1"
	if stepIndex := strings.Index(part, "/"); stepIndex >= 0 {
		rangePart, stepPart = part[:stepIndex], part[stepIndex+1:]
	}
	step, err := strconv.ParseUint(stepPart, 10, 32)
	if err != nil || step == 0 {
		return 0, fmt.Errorf("invalid step %q", stepPart)
	}

	lo, hi := min, max
	switch {
	case rangePart == "*" || rangePart == "":
	case strings.Contains(rangePart, "-"):
		bounds := strings.SplitN(rangePart, "-", 2)
		if len(bounds) != 2 {
			return 0, fmt.Errorf("invalid range %q", rangePart)
		}
		lower, err := strconv.ParseUint(bounds[0], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid range %q", rangePart)
		}
		upper, err := strconv.ParseUint(bounds[1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid range %q", rangePart)
		}
		lo, hi = uint(lower), uint(upper)
	default:
		value, err := strconv.ParseUint(rangePart, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid value %q", rangePart)
		}
		lo, hi = uint(value), uint(value)
		if strings.Contains(part, "/") {
			hi = max
		}
	}
	if lo < min || hi > max || lo > hi {
		return 0, fmt.Errorf("value outside range [%d,%d]", min, max)
	}
	var bits uint64
	for value := lo; value <= hi; value += uint(step) {
		bits |= 1 << value
	}
	return bits, nil
}

func (spec *CronSpec) Matches(now time.Time) bool {
	return spec.Minute&(1<<uint(now.Minute())) != 0 &&
		spec.Hour&(1<<uint(now.Hour())) != 0 &&
		spec.Month&(1<<uint(int(now.Month()))) != 0 &&
		spec.dayMatches(now)
}

func (spec *CronSpec) dayMatches(now time.Time) bool {
	dayHit := spec.Day&(1<<uint(now.Day())) != 0
	weekdayHit := spec.Weekday&(1<<uint(int(now.Weekday()))) != 0
	dayStar := spec.Day == (1<<32)-2
	weekdayStar := spec.Weekday == 127
	switch {
	case dayStar && weekdayStar:
		return true
	case dayStar:
		return weekdayHit
	case weekdayStar:
		return dayHit
	default:
		return dayHit || weekdayHit
	}
}

func (spec *CronSpec) Next(now time.Time) time.Time {
	cursor := now.Truncate(time.Minute).Add(time.Minute)
	limit := cursor.AddDate(5, 0, 0)
	for cursor.Before(limit) {
		if spec.Matches(cursor) {
			return cursor
		}
		cursor = cursor.Add(time.Minute)
	}
	return time.Time{}
}

func CompileSchedule(configs map[string][]ScheduleAction) map[string][]ScheduleEntry {
	entries := make(map[string][]ScheduleEntry, len(configs))
	for name, actions := range configs {
		for _, action := range actions {
			spec, err := ParseCron(action.Cron)
			if err != nil {
				continue
			}
			entries[name] = append(entries[name], ScheduleEntry{
				Action: action.Action,
				Cron:   action.Cron,
				Spec:   spec,
			})
		}
	}
	return entries
}

func NextSchedule(entries []ScheduleEntry, now time.Time) (time.Time, string) {
	var best time.Time
	action := ""
	for _, entry := range entries {
		next := entry.Spec.Next(now)
		if next.IsZero() {
			continue
		}
		if best.IsZero() || next.Before(best) {
			best, action = next, entry.Action
		}
	}
	return best, action
}
