// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package job

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type JobStartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 启动任务
func NewJobStartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobStartLogic {
	return &JobStartLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobStartLogic) JobStart(req *types.JobStartRequest) (*types.JobStartResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.FunctionID)
	if err != nil {
		return nil, err
	}

	if _, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req.Params)
	if err != nil {
		return nil, err
	}

	jobID, err := l.svcCtx.Dispatcher.StartJob(l.ctx, functionID, payload)
	if err != nil {
		return nil, err
	}

	return &types.JobStartResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"jobId": jobID,
		},
	}, nil
}
