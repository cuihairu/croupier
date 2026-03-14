package utils

import (
	"strings"
	"time"
)

var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02",
}

// ParseDate attempts to convert a string into a UTC timestamp.
func ParseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	var lastErr error
	for _, layout := range dateLayouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

// NormalizeDateRange ensures the start/end values are in chronological order and
// expands date-only end values to cover the entire day.
func NormalizeDateRange(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, err := ParseDate(startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := ParseDate(endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !end.IsZero() && end.Hour() == 0 && end.Minute() == 0 && end.Second() == 0 && end.Nanosecond() == 0 {
		end = end.Add(24 * time.Hour)
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		start, end = end, start
	}
	return start, end, nil
}
