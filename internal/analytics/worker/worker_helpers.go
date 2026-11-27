package worker

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// waitGroupBounded waits up to timeout for wg to finish. It returns true if
// the group completed, false on timeout (the waiter goroutine is abandoned;
// this is the bounded-exit cost when a refresh is genuinely stuck).
func waitGroupBounded(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func fmtAny(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(v)
	}
}

func asString(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

func asFloat(m map[string]any, k string) float64 {
	if v, ok := m[k]; ok {
		switch t := v.(type) {
		case float64:
			return t
		case int:
			return float64(t)
		}
	}
	return 0
}

// amountCentsUint64 parses payment amount_cents exactly (DATA-018): json.Number
// values go through ParseUint digit-for-digit with no float64 round-trip;
// legacy float64 payloads must be a positive integral within uint64 range.
// Fractional, negative, zero and out-of-range values are refused so they land
// in the dead letter with a reason instead of silently truncating or wrapping.
func amountCentsUint64(m map[string]any) (uint64, error) {
	v, ok := m["amount_cents"]
	if !ok {
		return 0, fmt.Errorf("amount_cents must be a positive integer")
	}
	switch t := v.(type) {
	case json.Number:
		n, err := strconv.ParseUint(t.String(), 10, 64)
		if err != nil || n == 0 {
			return 0, fmt.Errorf("amount_cents must be a positive integer")
		}
		return n, nil
	case float64:
		if t <= 0 || t != math.Trunc(t) || t > math.MaxUint64 {
			return 0, fmt.Errorf("amount_cents must be a positive integer")
		}
		return uint64(t), nil
	case int:
		if t <= 0 {
			return 0, fmt.Errorf("amount_cents must be a positive integer")
		}
		return uint64(t), nil
	case int64:
		if t <= 0 {
			return 0, fmt.Errorf("amount_cents must be a positive integer")
		}
		return uint64(t), nil
	default:
		return 0, fmt.Errorf("amount_cents must be a positive integer")
	}
}

// asRetryCount reads the message's retry_count field as an int. retry_count on external
// stream messages is a producer-controlled field: a valid value should be a float64 parsed
// from a JSON number, but a non-float64 type (string/bool/object, etc.) makes the .(float64)
// assertion panic — the worker Run loop has no recover, so a single bad message would crash
// the whole worker. This is fail-closed: a non-numeric type is treated as 0 (no panic) and a
// failed insert is re-queued with 0 retries; if the message itself already carries a bad type,
// the retry count cannot advance and cannot livelock either (each retry is treated as 0, and it
// still dead-letters within maxPendingRetries — the retry path must also normalize the type, see
// processMessage).
func asRetryCount(m map[string]any) int {
	if v, ok := m["retry_count"]; ok {
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case json.Number:
			if n, err := t.Int64(); err == nil {
				return int(n)
			}
		}
	}
	return 0
}

// parseEventTime is the single event-time contract for the analytics worker
// (DATA-054): the detail insert (normalizeTimestamp) and the aggregate touch
// (touchAgg) must derive the same instant from the same payload, so one old
// or replayed event always produces a historical raw row AND the matching
// historical bucket instead of a processing-time bucket that drifts on every
// replay. Accepted wire formats: RFC3339 with any offset (normalized to UTC,
// which is what business-timezone producers send), the ClickHouse datetime
// layout itself (assumed UTC, matching the flush-side parse), and unix epoch
// seconds/milliseconds. ok=false is the explicit invalid disposition: empty
// or unrecognized input reaches neither path — the raw insert passes the raw
// value through (ClickHouse Append errors and the message retries into the
// dead letter) and the aggregation skips the event.
func parseEventTime(raw string) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse("2006-01-02 15:04:05", trimmed); err == nil {
		return t.UTC(), true
	}
	if secs, err := strconv.ParseInt(trimmed, 10, 64); err == nil && secs > 0 {
		if secs > 1e12 { // 13+ digits: milliseconds (quick-start producers)
			secs /= 1000
		}
		return time.Unix(secs, 0).UTC(), true
	}
	return time.Time{}, false
}

// normalizeTimestamp coerces the wire formats parseEventTime accepts into the
// ClickHouse datetime layout "2006-01-02 15:04:05" expected by appendEventRow.
func normalizeTimestamp(raw string) string {
	if t, ok := parseEventTime(raw); ok {
		return t.UTC().Format("2006-01-02 15:04:05")
	}
	// Invalid (DATA-054 explicit disposition): pass the raw value through;
	// ClickHouse Append will surface a parse error (and the message retries
	// into the dead letter). Empty input previously fell back to time.Now(),
	// which stamped a different event_time on every replay — that fallback
	// is gone.
	return strings.TrimSpace(raw)
}
