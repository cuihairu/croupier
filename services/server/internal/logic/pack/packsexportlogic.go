// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PacksExportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 导出功能包
func NewPacksExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksExportLogic {
	return &PacksExportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksExportLogic) PacksExport(req *types.PacksExportRequest) (resp *types.PacksExportResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
