// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"
	"io"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type PacksExportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPacksExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksExportLogic {
	return &PacksExportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksExportLogic) PacksExport(w io.Writer) error {
	if w == nil {
		return errors.New("nil writer")
	}
	packDir := l.svcCtx.PackDir()
	if packDir == "" {
		return errors.New("pack directory not configured")
	}
	return streamPackArchive(w, packDir)
}

func (l *PacksExportLogic) PackETag() string {
	return computePackETag(l.svcCtx.PackDir())
}
