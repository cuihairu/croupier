package journal

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/filter"
)

// CaptureStats describes one consumed source stream.
type CaptureStats struct {
	Seen     uint64
	Selected uint64
	Acked    uint64
}

// Capture polls a source, journals selected events, then acknowledges each
// event's source position. Excluded events are intentionally acknowledged
// without journaling. If append fails, that event is never acknowledged; if
// acknowledgement fails, a later redelivery is safe because Append is
// idempotent. The caller owns source and journal lifetimes.
func Capture(ctx context.Context, source cdc.SourceConnector, log *Journal, selection filter.Filter, request cdc.PollRequest) (CaptureStats, error) {
	var stats CaptureStats
	if source == nil || log == nil {
		return stats, fmt.Errorf("journal: source and journal are required")
	}
	events, err := source.Poll(ctx, request)
	if err != nil {
		return stats, fmt.Errorf("journal: poll source: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		case event, ok := <-events:
			if !ok {
				return stats, nil
			}
			stats.Seen++
			if !event.Position.Valid() {
				return stats, fmt.Errorf("journal: source event has invalid position kind %q", event.Position.Kind)
			}
			if selection.Match(event) {
				if _, _, err := log.Append(event); err != nil {
					return stats, fmt.Errorf("journal: append source event: %w", err)
				}
				stats.Selected++
			}
			if err := source.AckCheckpoint(ctx, event.Position); err != nil {
				return stats, fmt.Errorf("journal: acknowledge source position %s: %w", event.Position.String(), err)
			}
			stats.Acked++
		}
	}
}
