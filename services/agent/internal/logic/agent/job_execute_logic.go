// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type JobExecuteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJobExecuteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobExecuteLogic {
	return &JobExecuteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobExecuteLogic) JobExecute(req *types.JobExecuteRequest) (*types.JobExecuteResponse, error) {
	if req == nil {
		return nil, errors.New("missing request payload")
	}

	jobID := strings.TrimSpace(req.JobId)
	if jobID == "" {
		jobID = fmt.Sprintf("job-%d", time.Now().UnixNano())
	}

	now := time.Now()
	result := map[string]interface{}{
		"echo":    svc.CloneInterfaceMap(req.Inputs),
		"options": svc.CloneInterfaceMap(req.Options),
	}

	state := l.svcCtx.AgentState
	state.Mu.Lock()
	state.Jobs[jobID] = &svc.JobRecord{
		ID:         jobID,
		FunctionID: strings.TrimSpace(req.FunctionId),
		GameID:     strings.TrimSpace(req.GameId),
		Env:        strings.TrimSpace(req.Env),
		Status:     "completed",
		Result:     result,
		StartTime:  now,
		EndTime:    func() *time.Time { t := now; return &t }(),
	}
	state.Mu.Unlock()

	return &types.JobExecuteResponse{
		Success: true,
		JobId:   jobID,
		Status:  "completed",
	}, nil
}
