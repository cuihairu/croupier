package incident

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"github.com/cuihairu/croupier/internal/svc"
)

// IncidentLifecycleKind 生命周期事件类型闭集（M2 风格信封）。
type IncidentLifecycleKind string

const (
	LifecycleIncidentCreated   IncidentLifecycleKind = "incident.created"
	LifecycleIncidentEscalated IncidentLifecycleKind = "incident.escalated"
	LifecycleIncidentResolved  IncidentLifecycleKind = "incident.resolved"
)

// IncidentLifecycleEvent M2 风格生命周期信封。
type IncidentLifecycleEvent struct {
	Kind    IncidentLifecycleKind
	EventID string
	Title   string
	Body    string
}

// DispatchLifecycle 异步推一条生命周期事件到外部出口链（§9.3）。
// OutletManager 未布线时静默跳过；失败仅记日志不抛错。
func DispatchLifecycle(svcCtx *svc.ServiceContext, row *model.Incident, kind IncidentLifecycleKind) {
	if svcCtx == nil || svcCtx.OutletManager == nil || row == nil {
		return
	}
	ev := lifecycleEvent(row, kind)
	if ev == nil {
		return
	}
	go func() {
		ctx := context.Background()
		evOut := outlet.AlertEvent{
			Kind: "incident-lifecycle",
			// 严重度直传：model 与 outlet 的取值同为 info/warning/critical
			Severity:         row.Severity,
			EventID:          ev.EventID,
			DedupKey:         ev.EventID,
			Title:            ev.Title,
			Body:             ev.Body,
			OccurredAtUnixMs: time.Now().UnixMilli(),
		}
		_, _ = svcCtx.OutletManager.DispatchExternal(ctx, evOut)
	}()
}

// lifecycleEvent 构造生命周期事件。
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

// severityRank 严重度排序（用于升级检测）。
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
