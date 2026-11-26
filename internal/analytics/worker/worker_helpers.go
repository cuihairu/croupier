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

// normalizeTimestamp coerces common wire formats into the ClickHouse
// datetime layout "2006-01-02 15:04:05" expected by appendEventRow:
// RFC3339 (bridge/E2E producers), unix seconds, and the layout itself.
func normalizeTimestamp(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Now().Format("2006-01-02 15:04:05")
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t.UTC().Format("2006-01-02 15:04:05")
	}
	if _, err := time.Parse("2006-01-02 15:04:05", trimmed); err == nil {
		return trimmed
	}
	if secs, err := strconv.ParseInt(trimmed, 10, 64); err == nil && secs > 0 {
		if secs > 1e12 { // 13+ digits: milliseconds (quick-start producers)
			secs /= 1000
		}
		return time.Unix(secs, 0).UTC().Format("2006-01-02 15:04:05")
	}
	// Unrecognized: fall through with the raw value; ClickHouse Append will
	// surface a parse error (and the message retries into the dead letter).
	return trimmed
}
