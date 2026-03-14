// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type RenameDirectoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重命名/移动目录
func NewRenameDirectoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RenameDirectoryLogic {
	return &RenameDirectoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RenameDirectoryLogic) RenameDirectory(req *types.RenameDirectoryRequest) (resp *types.RenameDirectoryResponse, err error) {
	return nil, errorx.NewNotImplemented("RenameDirectory not implemented")
}
