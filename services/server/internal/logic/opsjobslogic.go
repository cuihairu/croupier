package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsJobsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsJobsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsJobsLogic {
	return &OpsJobsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsJobsLogic) OpsJobs(req *types.OpsJobsQuery) (*types.OpsJobsResponse, error) {
	page := 1
	size := 20
	var status, fid, actor, gid, env string
	if req != nil {
		if req.Page > 0 {
			page = req.Page
		}
		if req.Size > 0 {
			size = req.Size
		}
		status = strings.TrimSpace(req.Status)
		fid = strings.TrimSpace(req.FunctionId)
		actor = strings.TrimSpace(req.Actor)
		gid = strings.TrimSpace(req.GameId)
		env = strings.TrimSpace(req.Env)
	}
	if size > 200 {
		size = 200
	}
	if size <= 0 {
		size = 20
	}
	data, order := l.svcCtx.JobsSnapshot()
	filtered := make([]*svc.JobInfo, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		ji := data[id]
		if ji == nil {
			continue
		}
		if status != "" && ji.State != status {
			continue
		}
		if fid != "" && ji.FunctionID != fid {
			continue
		}
		if actor != "" && ji.Actor != actor {
			continue
		}
		if gid != "" && ji.GameID != gid {
			continue
		}
		if env != "" && ji.Env != env {
			continue
		}
		filtered = append(filtered, ji)
	}
	total := len(filtered)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	window := filtered[start:end]
	resp := &types.OpsJobsResponse{
		Jobs:  make([]types.OpsJob, 0, len(window)),
		Total: total,
		Page:  page,
		Size:  size,
	}
	for _, ji := range window {
		resp.Jobs = append(resp.Jobs, types.OpsJob{
			Id:         ji.ID,
			FunctionId: ji.FunctionID,
			Actor:      ji.Actor,
			GameId:     ji.GameID,
			Env:        ji.Env,
			State:      ji.State,
			StartedAt:  formatRFC3339(ji.StartedAt),
			EndedAt:    formatRFC3339(ji.EndedAt),
			DurationMs: ji.DurationMs,
			Error:      ji.Error,
			RpcAddr:    ji.RPCAddr,
			TraceId:    ji.TraceID,
		})
	}
	return resp, nil
}
