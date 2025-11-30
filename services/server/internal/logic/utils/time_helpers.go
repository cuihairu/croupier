package utils

import "time"

// FormatTimestamp normalizes timestamps for API responses.
func FormatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// FormatTimestampPtr formats nullable timestamps.
func FormatTimestampPtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return FormatTimestamp(*t)
}
