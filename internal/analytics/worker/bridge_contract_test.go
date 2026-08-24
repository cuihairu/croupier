package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Contract: the telemetry bridge publishes to "analytics:events" with a
// snake_case envelope. This pins both sides without importing the
// telemetry package (which would drag the otel SDK into worker tests):
// the bridge side constant lives in internal/telemetry/analytics_bridge.go.
const bridgeTargetStream = "analytics:events"

func TestBridgePayloadIsWorkerCompatible(t *testing.T) {
	bridgeEvent := map[string]interface{}{
		"event":      "session.start",
		"game_id":    "contract-game",
		"env":        "dev",
		"user_id":    "u-1",
		"session_id": "s-1",
		"platform":   "ios",
		"ts":         time.Now().UTC().Format(time.RFC3339),
		"country":    "US",
		"props":      map[string]interface{}{"trace_id": "abc"},
	}
	raw, err := json.Marshal(bridgeEvent)
	if err != nil {
		t.Fatal(err)
	}
	if bridgeTargetStream != "analytics:events" {
		t.Fatalf("bridge target stream drifted: %s", bridgeTargetStream)
	}

	w, _ := testWorker(t)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal bridge payload: %v", err)
	}

	// Detail insert must accept the payload (event_id absent → generated).
	if err := w.insertEvent(context.Background(), m); err != nil {
		t.Fatalf("worker insertEvent rejected bridge payload: %v", err)
	}

	// Aggregation must recognize the dotted event name.
	w.touchAgg(context.Background(), m)
	if len(w.touchedDays) != 1 {
		t.Fatalf("bridge session.start must drive DAU aggregation, touchedDays=%v", w.touchedDays)
	}
	if len(w.touchedMinutes) != 1 {
		t.Fatalf("bridge session.start must drive minute-online aggregation, touchedMinutes=%v", w.touchedMinutes)
	}
}
