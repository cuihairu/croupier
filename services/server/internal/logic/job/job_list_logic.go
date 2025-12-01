// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package job

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type JobListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 任务列表
func NewJobListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobListLogic {
	return &JobListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobListLogic) JobList(_ *types.JobListRequest) (*types.JobListResponse, error) {
	return &types.JobListResponse{
		Jobs:  []types.JobItem{},
		Total: 0,
	}, nil
}
