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

// toDTO converts from model.WorkspaceConfig to types.WorkspaceConfig
func toDTO(m *model.WorkspaceConfig) types.WorkspaceConfig {
	cfg := types.WorkspaceConfig{
		ObjectKey:   m.ObjectKey,
		Title:       m.Title,
		Published:   m.Published,
		MenuOrder:   int(m.MenuOrder),
		Status:      "draft",
		PublishedBy: m.PublishedBy,
	}

	if m.PublishedAt != nil {
		cfg.PublishedAt = m.PublishedAt.Format(time.RFC3339)
	}

	// Unmarshal the JSON config into Layout
	if len(m.Config) > 0 {
		var layout interface{}
		if err := json.Unmarshal(m.Config, &layout); err == nil {
			cfg.Layout = layout
		}
	}

	if m.Published {
		cfg.Status = "published"
	}

	return cfg
}

// parseWorkspaceVersionTimeRange parses from/to time strings into time.Time values
func parseWorkspaceVersionTimeRange(from, to string) (fromAt, toAt time.Time, err error) {
	if from = strings.TrimSpace(from); from != "" {
		fromAt, err = time.Parse(time.RFC3339, from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'from' time format: %w", err)
		}
	}
	if to = strings.TrimSpace(to); to != "" {
		toAt, err = time.Parse(time.RFC3339, to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'to' time format: %w", err)
		}
	}
	return fromAt, toAt, nil
}
