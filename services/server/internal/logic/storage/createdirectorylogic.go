// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateDirectoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建目录
func NewCreateDirectoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateDirectoryLogic {
	return &CreateDirectoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateDirectoryLogic) CreateDirectory(req *types.CreateDirectoryRequest) (resp *types.CreateDirectoryResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
