package service

import (
	"context"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func (s *ProposalService) publishPermission(ctx context.Context) (string, error) {
	if s.permissionGuard == nil {
		return actorFromContext(ctx), nil
	}
	actor, err := s.permissionGuard(ctx, actorFromContext(ctx))
	if err != nil {
		return "", err
	}
	return actor, nil
}

func serviceProposalPermissionGuard(s *ProposalService) func(context.Context, string) (string, error) {
	return func(ctx context.Context, actor string) (string, error) {
		if err := s.requireProposalPublishPermission(ctx); err != nil {
			return "", err
		}
		return actor, nil
	}
}

func InternalTestProposalPermissionGuard() func(context.Context, string) (string, error) {
	return func(ctx context.Context, actor string) (string, error) {
		return actor, nil
	}
}

func (s *ProposalService) requireProposalPublishPermission(ctx context.Context) error {
	resolvedDB := dbctx.Resolve(ctx, s.db)
	_, _, err := logicutils.RequireAnyPermission(ctx, &svc.ServiceContext{
		DB:              resolvedDB,
		AdminModel:      model.NewAdminModel(resolvedDB),
		RoleModel:       model.NewRoleModel(resolvedDB),
		PermissionModel: model.NewPermissionModel(resolvedDB),
	}, "无权发布页面", "admin:all", "pages:publish")
	return err
}
