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
	if err := selection.Validate(); err != nil {
		return stats, err
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
			selected, err := CaptureEvent(ctx, source, log, selection, event, nil)
			if err != nil {
				return stats, err
			}
			if selected {
				stats.Selected++
			}
			stats.Acked++
		}
	}
}

// CaptureEvent journals a selected event, invokes the optional consumer, and
// acknowledges its source position last. This lets an application share one
// source stream between CDC retention and its existing event processing path.
// A nil journal disables persistence but keeps consumer and acknowledgement
// ordering intact.
func CaptureEvent(ctx context.Context, source cdc.SourceConnector, log *Journal, selection filter.Filter, event cdc.ChangeEvent, consume func(cdc.ChangeEvent) error) (bool, error) {
	if source == nil {
		return false, fmt.Errorf("journal: source is required")
	}
	if log != nil {
		if err := selection.Validate(); err != nil {
			return false, err
		}
	}
	if !event.Position.Valid() {
		return false, fmt.Errorf("journal: source event has invalid position kind %q", event.Position.Kind)
	}
	selected := log != nil && selection.Match(event)
	if selected {
		if _, _, err := log.Append(event); err != nil {
			return false, fmt.Errorf("journal: append source event: %w", err)
		}
	}
	if consume != nil {
		if err := consume(event); err != nil {
			return selected, fmt.Errorf("journal: consume source event: %w", err)
		}
	}
	if err := source.AckCheckpoint(ctx, event.Position); err != nil {
		return selected, fmt.Errorf("journal: acknowledge source position %s: %w", event.Position.String(), err)
	}
	return selected, nil
}
