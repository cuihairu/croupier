// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PacksImportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPacksImportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksImportLogic {
	return &PacksImportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksImportLogic) PacksImport(tmpPath string) (*types.PacksImportResponse, error) {
	if tmpPath == "" {
		return nil, errors.New("missing pack file")
	}
	packDir := l.svcCtx.PackDir()
	if packDir == "" {
		return nil, errors.New("pack directory not configured")
	}
	if err := extractPackArchive(tmpPath, packDir); err != nil {
		return nil, err
	}
	l.svcCtx.ReloadDescriptors()
	return &types.PacksImportResponse{Ok: true}, nil
}
