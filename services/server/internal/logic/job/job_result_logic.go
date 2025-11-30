// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package job

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type JobResultLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取任务结果
func NewJobResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobResultLogic {
	return &JobResultLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobResultLogic) JobResult(req *types.JobResultRequest) (*types.JobResultResponse, error) {
	jobID := strings.TrimSpace(req.ID)
	if jobID == "" {
		return nil, fmt.Errorf("任务ID不能为空")
	}

	events, done, err := l.svcCtx.Dispatcher.StreamJob(l.ctx, jobID)
	if err != nil {
		return nil, err
	}

	return &types.JobResultResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"jobId":  jobID,
			"done":   done,
			"events": events,
		},
	}, nil
}
