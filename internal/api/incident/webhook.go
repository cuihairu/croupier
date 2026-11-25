package incident

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"github.com/cuihairu/croupier/internal/svc"
)

// IncidentLifecycleKind is the closed set of lifecycle event kinds (M2-style envelope).
type IncidentLifecycleKind string

const (
	LifecycleIncidentCreated   IncidentLifecycleKind = "incident.created"
	LifecycleIncidentEscalated IncidentLifecycleKind = "incident.escalated"
	LifecycleIncidentResolved  IncidentLifecycleKind = "incident.resolved"
)

// IncidentLifecycleEvent is the M2-style lifecycle envelope.
type IncidentLifecycleEvent struct {
	Kind    IncidentLifecycleKind
	EventID string
	Title   string
	Body    string
}

// DispatchLifecycle pushes one lifecycle event asynchronously to the external
// outlet chain (§9.3). Skips silently when OutletManager is not wired; failures
// are only logged, never propagated.
func DispatchLifecycle(ctx context.Context, svcCtx *svc.ServiceContext, row *model.Incident, kind IncidentLifecycleKind) {
	if svcCtx == nil || svcCtx.OutletManager == nil || row == nil {
		return
	}
	ev := lifecycleEvent(row, kind)
	if ev == nil {
		return
	}
	// WithoutCancel derives from the input ctx: the push is decoupled from
	// the request lifecycle (client disconnect does not cancel the outgoing
	// send) while ctx values (game DB routing etc.) are kept (CR-FIX-22).
	go func() {
		dctx := context.WithoutCancel(ctx)
		evOut := outlet.AlertEvent{
			Kind: "incident-lifecycle",
			// Severity passed through as-is: model and outlet share the same
			// info/warning/critical values
			Severity:         row.Severity,
			EventID:          ev.EventID,
			DedupKey:         ev.EventID,
			Title:            ev.Title,
			Body:             ev.Body,
			OccurredAtUnixMs: time.Now().UnixMilli(),
		}
		_, _ = svcCtx.OutletManager.DispatchExternal(dctx, evOut)
	}()
}

// lifecycleEvent builds a lifecycle event.
func lifecycleEvent(row *model.Incident, kind IncidentLifecycleKind) *IncidentLifecycleEvent {
	eventID := fmt.Sprintf("incident:%d:%s", row.ID, string(kind))
	title := fmt.Sprintf("[%s] %s", kind, row.Title)
	body := fmt.Sprintf("title=%s severity=%s status=%s category=%d", row.Title, row.Severity, row.Status, row.CategoryID)
	return &IncidentLifecycleEvent{
		Kind:    kind,
		EventID: eventID,
		Title:   title,
		Body:    body,
	}
}

// severityRank orders severities (used for escalation detection).
func severityRank(v string) int {
	switch v {
	case model.IncidentSeverityInfo:
		return 1
	case model.IncidentSeverityWarning:
		return 2
	case model.IncidentSeverityCritical:
		return 3
	default:
		return 0
	}
}
