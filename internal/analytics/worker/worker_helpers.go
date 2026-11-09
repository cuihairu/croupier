package worker

import (
	"encoding/json"
	"fmt"
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

// asRetryCount 读取消息的 retry_count 字段为 int。外部流消息的 retry_count
// 是生产方可控字段：合法值应是从 JSON 数字解析的 float64，但非 float64 类型
// （字符串/布尔/对象等）会令 .(float64) 断言 panic——worker Run 循环无
// recover，单条坏消息会崩掉整个 worker。此处 fail-closed：非数值类型视为
// 0（不 panic），让插入失败后按 0 次重试入队；若消息本身已带坏类型，重试
// 计数不推进也不会死循环（每次重试都按 0 处理，仍会在 maxPendingRetries
// 内进入死信——需在 retry 路径同时做类型归一，见 processMessage）。
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
