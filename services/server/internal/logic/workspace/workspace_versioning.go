package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

func workspaceVersionKey(objectKey string) string {
	return fmt.Sprintf("workspace:%s", strings.TrimSpace(objectKey))
}

func workspaceActorFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if username, ok := ctx.Value("username").(string); ok {
		return strings.TrimSpace(username)
	}
	return ""
}

func workspaceRequestIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value("workspaceRequestID").(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func persistWorkspaceVersion(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	cfg types.WorkspaceConfig,
	actor string,
	comment string,
) (int, error) {
	if svcCtx == nil || svcCtx.ConfigVersionModel == nil {
		return 0, nil
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return 0, err
	}
	record, err := svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:     workspaceVersionKey(cfg.ObjectKey),
		Content: string(payload),
		Format:  "json",
		Message: comment,
	}, strings.TrimSpace(actor))
	if err != nil {
		return 0, err
	}
	return record.Version, nil
}

func parseWorkspaceDTOFromVersion(value string) (*types.WorkspaceConfig, error) {
	var cfg types.WorkspaceConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func parseTimePtr(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &parsed
}
