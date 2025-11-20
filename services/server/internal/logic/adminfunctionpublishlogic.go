package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionPublishLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminFunctionPublishLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionPublishLogic {
	return &AdminFunctionPublishLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionPublishLogic) AdminFunctionPublish(req *types.AdminPublishRequest) (*types.AdminPublishResponse, error) {
	if req == nil || strings.TrimSpace(req.Fid) == "" {
		return nil, errors.New("function id required")
	}
	meta := l.svcCtx.RegistryStore.BuildFunctionIndex()
	data, ok := meta[req.Fid]
	if !ok || data == nil {
		return nil, errors.New("no manifest metadata")
	}
	entry := map[string]any{}
	for _, k := range []string{"display_name", "summary", "tags", "menu", "permissions"} {
		if v, ok := data[k]; ok && v != nil {
			entry[k] = v
		}
	}
	if len(entry) == 0 {
		entry["display_name"] = map[string]string{"zh": req.Fid}
	}
	if err := l.svcCtx.SaveUIOverride(req.Fid, entry); err != nil {
		return nil, err
	}
	return &types.AdminPublishResponse{Ok: true}, nil
}
