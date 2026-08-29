package telemetry

import (
	"testing"
	"time"
)

// TestWorkerPayloadFromEventTimestampUnit 锁定 ts 单位契约：
// AnalyticsEvent.Timestamp 是 UnixMilli，payload ts 必须还原为毫秒精度
// RFC3339——按秒解析会把事件抛到 5 万年后（worker ClickHouse 写入
// 全部失败进死信，生产实测过）。
func TestWorkerPayloadFromEventTimestampUnit(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond).UTC()
	evt := AnalyticsEvent{
		EventType: "function.call",
		GameID:    "demo",
		Timestamp: now.UnixMilli(),
	}

	payload := workerPayloadFromEvent(evt)

	raw, ok := payload["ts"].(string)
	if !ok {
		t.Fatalf("payload ts 不是 string: %T", payload["ts"])
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("ts 不是 RFC3339: %v", err)
	}
	if diff := parsed.Sub(now); diff < -time.Second || diff > time.Second {
		t.Fatalf("ts 单位错位: got %s want ~%s (diff %s)", parsed, now, diff)
	}

	if payload["event"] != "function.call" || payload["game_id"] != "demo" {
		t.Fatalf("基础字段丢失: %+v", payload)
	}
}
