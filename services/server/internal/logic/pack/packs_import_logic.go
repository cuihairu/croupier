// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type PacksImportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 导入功能包
func NewPacksImportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksImportLogic {
	return &PacksImportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksImportLogic) PacksImport(req *types.PacksImportRequest) (*types.PacksImportResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权导入功能包", "admin:all", "packs:reload"); err != nil {
		return nil, err
	}

	if req == nil || req.Archive == "" {
		return nil, errors.New("archive payload is required")
	}

	data, err := base64.StdEncoding.DecodeString(req.Archive)
	if err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(l.svcCtx.Config)
	destDir := filepath.Join(packsDir, "dist")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}

	if err := extractArchive(data, destDir); err != nil {
		return nil, err
	}

	return &types.PacksImportResponse{
		Message: "Imported",
	}, nil
}
