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

func (l *PacksExportLogic) PacksExport(_ *types.PacksExportRequest) (*types.PacksExportResponse, error) {
	packsDir := resolvePacksDir(l.svcCtx.Config)
	filename, data, err := buildPacksArchive(packsDir)
	if err != nil {
		return nil, err
	}

	return &types.PacksExportResponse{
		Filename:    filename,
		ContentType: "application/gzip",
		Content:     data,
	}, nil
}
