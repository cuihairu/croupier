package workspace

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
)

func requireWorkspacePermission(ctx context.Context, svcCtx *svc.ServiceContext, action string) error {
	if svcCtx == nil || svcCtx.PermissionService == nil {
		return nil
	}

	adminID, err := getAdminIDFromContext(ctx)
	if err != nil {
		return err
	}

	candidates := workspacePermissionCandidates(action)
	for i := range candidates {
		has, checkErr := svcCtx.PermissionService.CheckPermission(
			ctx,
			adminID,
			candidates[i].resource,
			candidates[i].action,
		)
		if checkErr != nil {
			continue
		}
		if has {
			return nil
		}
	}

	return errorx.NewForbidden("permission denied")
}

func getAdminIDFromContext(ctx context.Context) (uint, error) {
	if ctx == nil {
		return 0, errorx.NewUnauthorized("admin not found")
	}
	if v := ctx.Value("adminID"); v != nil {
		switch id := v.(type) {
		case uint:
			if id > 0 {
				return id, nil
			}
		case int64:
			if id > 0 {
				return uint(id), nil
			}
		}
	}
	return 0, errorx.NewUnauthorized("admin not found")
}

type workspacePermissionCandidate struct {
	resource string
	action   string
}

func workspacePermissionCandidates(action string) []workspacePermissionCandidate {
	switch action {
	case "read":
		return []workspacePermissionCandidate{
			{resource: "workspace", action: "read"},
			{resource: "workspaces", action: "read"},
			{resource: "config", action: "read"},
		}
	case "edit":
		return []workspacePermissionCandidate{
			{resource: "workspace", action: "edit"},
			{resource: "workspace", action: "update"},
			{resource: "workspaces", action: "edit"},
			{resource: "workspaces", action: "update"},
			{resource: "config", action: "edit"},
			{resource: "config", action: "update"},
		}
	case "publish":
		return []workspacePermissionCandidate{
			{resource: "workspace", action: "publish"},
			{resource: "workspaces", action: "publish"},
			{resource: "config", action: "publish"},
		}
	case "rollback":
		return []workspacePermissionCandidate{
			{resource: "workspace", action: "rollback"},
			{resource: "workspaces", action: "rollback"},
			{resource: "config", action: "rollback"},
			{resource: "workspace", action: "update"},
			{resource: "config", action: "update"},
		}
	case "delete":
		return []workspacePermissionCandidate{
			{resource: "workspace", action: "delete"},
			{resource: "workspaces", action: "delete"},
			{resource: "config", action: "delete"},
		}
	default:
		return []workspacePermissionCandidate{
			{resource: "workspace", action: action},
			{resource: "workspaces", action: action},
			{resource: "config", action: action},
		}
	}
}
