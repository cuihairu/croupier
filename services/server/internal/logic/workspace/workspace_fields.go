package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"gorm.io/gorm"
)

const (
	workspaceStatusDraft     = "draft"
	workspaceStatusPublished = "published"
	workspaceStatusArchived  = "archived"
)

func normalizeWorkspaceStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case workspaceStatusDraft:
		return workspaceStatusDraft
	case workspaceStatusPublished:
		return workspaceStatusPublished
	case workspaceStatusArchived:
		return workspaceStatusArchived
	default:
		return ""
	}
}

func resolveWorkspaceStatus(config *types.WorkspaceConfig) string {
	if config == nil {
		return workspaceStatusDraft
	}
	if config.Published {
		return workspaceStatusPublished
	}
	switch normalizeWorkspaceStatus(config.Status) {
	case workspaceStatusArchived:
		return workspaceStatusArchived
	case workspaceStatusDraft:
		return workspaceStatusDraft
	default:
		return workspaceStatusDraft
	}
}

func enrichWorkspaceVersion(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	config *types.WorkspaceConfig,
) error {
	if config == nil || svcCtx == nil || svcCtx.ConfigVersionModel == nil {
		return nil
	}
	latest, err := svcCtx.ConfigVersionModel.FindLatest(ctx, workspaceVersionKey(config.ObjectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			config.Version = 0
			return nil
		}
		return err
	}
	config.Version = latest.Version
	return nil
}
