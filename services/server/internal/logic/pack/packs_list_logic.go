// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type PacksListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取功能包列表
func NewPacksListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksListLogic {
	return &PacksListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksListLogic) PacksList(_ *types.PacksListRequest) (*types.PacksListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看功能包列表", "admin:all", "packs:list", "packs:read", "packs:reload"); err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(l.svcCtx.Config)
	summaries, err := loadPackSummaries(packsDir)
	if err != nil {
		return nil, err
	}

	manifest, packEntries := aggregateManifest(summaries)
	counts := map[string]int{
		"descriptors": 0,
		"ui_schema":   0,
	}
	for _, summary := range summaries {
		counts["descriptors"] += summary.DescriptorCount
		counts["ui_schema"] += summary.UISchemaCount
	}

	manifestBytes, _ := json.Marshal(manifest)
	etag := ""
	if len(manifestBytes) > 0 {
		sum := sha256.Sum256(manifestBytes)
		etag = fmt.Sprintf("%x", sum[:])
	}

	return &types.PacksListResponse{
		Manifest:           manifest,
		Packs:              packEntries,
		Counts:             counts,
		ETag:               etag,
		ExportAuthRequired: false,
	}, nil
}
