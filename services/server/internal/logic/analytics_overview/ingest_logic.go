// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 采集分析数据
func NewIngestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngestLogic {
	return &IngestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngestLogic) Ingest(req *types.IngestRequest) (resp *types.IngestResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
