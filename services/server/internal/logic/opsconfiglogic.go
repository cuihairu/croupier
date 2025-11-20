package logic

import (
	"context"
	"os"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsConfigLogic {
	return &OpsConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsConfigLogic) OpsConfig() (*types.OpsConfigResponse, error) {
	return &types.OpsConfigResponse{
		AlertmanagerUrl:   strings.TrimSpace(os.Getenv("ALERTMANAGER_URL")),
		GrafanaExploreUrl: strings.TrimSpace(os.Getenv("GRAFANA_EXPLORE_URL")),
	}, nil
}
