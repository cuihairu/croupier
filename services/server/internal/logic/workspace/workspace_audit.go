package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
)

func appendWorkspaceAudit(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	action string,
	objectKey string,
	result string,
	metadata map[string]interface{},
) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return
	}
	actor := workspaceActorFromCtx(ctx)
	if actor == "" {
		actor = "unknown"
	}
	requestID := workspaceRequestIDFromCtx(ctx)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if requestID != "" {
		metadata["request_id"] = requestID
	}
	metadata["objectKey"] = strings.TrimSpace(objectKey)

	now := time.Now().UTC()
	_, _ = svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
		if state == nil {
			return
		}
		state.Audit.Entries = append(state.Audit.Entries, svc.OpsAuditEntry{
			ID:        fmt.Sprintf("workspace-%d", now.UnixNano()),
			Action:    action,
			UserID:    actor,
			Target:    strings.TrimSpace(objectKey),
			Result:    strings.TrimSpace(result),
			TraceID:   requestID,
			Metadata:  metadata,
			CreatedAt: now,
		})
		state.Audit.UpdatedAt = now
	})
}
