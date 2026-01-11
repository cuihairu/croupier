// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package job

import (
	"context"
	"fmt"
	"strings"

	functionv1 "github.com/cuihairu/croupier/generated/croupier/function/v1"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StreamJobLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 任务流（实时状态和日志）
func NewStreamJobLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamJobLogic {
	return &StreamJobLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StreamJobLogic) StreamJob(req *types.StreamJobRequest) (*types.StreamJobResponse, error) {
	jobID := strings.TrimSpace(req.JobID)
	if jobID == "" {
		return nil, fmt.Errorf("任务ID不能为空")
	}

	result := make([]map[string]interface{}, 0)
	done, err := l.svcCtx.Dispatcher.StreamJobRealtime(l.ctx, jobID, func(evt *functionv1.JobEvent) bool {
		if evt == nil {
			return true
		}
		result = append(result, map[string]interface{}{
			"type":     evt.Type,
			"message":  evt.Message,
			"progress": evt.GetProgress(),
			"payload":  evt.Payload,
		})
		return true
	})

	if err != nil {
		return nil, err
	}

	return &types.StreamJobResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"events": result,
			"done":   done,
		},
	}, nil
}
