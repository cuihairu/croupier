// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type WorkspaceConfigGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取单个 Workspace 配置
func NewWorkspaceConfigGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceConfigGetLogic {
	return &WorkspaceConfigGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceConfigGetLogic) WorkspaceConfigGet(req *types.WorkspaceConfigGetRequest) (resp *types.WorkspaceConfigGetResponse, err error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	cfg, err := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace config not found")
		}
		return nil, err
	}
	dto := toDTO(cfg)
	_ = enrichWorkspaceVersion(l.ctx, l.svcCtx, &dto)
	return &types.WorkspaceConfigGetResponse{WorkspaceConfig: dto}, nil
}
