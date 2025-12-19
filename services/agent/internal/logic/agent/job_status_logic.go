// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type JobStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJobStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobStatusLogic {
	return &JobStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobStatusLogic) JobStatus(req *types.JobStatusRequest) (*types.JobStatusResponse, error) {
	if req == nil || strings.TrimSpace(req.JobId) == "" {
		return nil, errors.New("job_id 不能为空")
	}
	if l.svcCtx.Core == nil || l.svcCtx.Core.Store() == nil {
		return nil, errors.New("agent core is not running")
	}

	jobID := strings.TrimSpace(req.JobId)
	result, ok := l.svcCtx.Core.Store().GetJobResult(jobID)
	if !ok || result == nil {
		return nil, errors.New("job not found")
	}

	out := map[string]interface{}{}
	if len(result.Payload) > 0 {
		var any interface{}
		if err := json.Unmarshal(result.Payload, &any); err == nil {
			out["payload"] = any
		} else {
			out["payload_base64"] = base64.StdEncoding.EncodeToString(result.Payload)
		}
	}

	progress := int64(0)
	switch strings.ToLower(result.State) {
	case "pending":
		progress = 0
	case "running":
		progress = 50
	default:
		progress = 100
	}

	return &types.JobStatusResponse{
		JobId:     result.JobID,
		Status:    result.State,
		Result:    out,
		Error:     result.Error,
		Progress:  progress,
		StartTime: formatTime(result.CreatedAt),
		EndTime:   formatTime(result.UpdatedAt),
	}, nil
}
