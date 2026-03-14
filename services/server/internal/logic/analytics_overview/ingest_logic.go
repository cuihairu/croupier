// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type IngestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 采集分析数据
func NewIngestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngestLogic {
	return &IngestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngestLogic) Ingest(req *types.IngestRequest) (*types.IngestResponse, error) {
	if l.svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("gameId 不能为空")
	}
	env := strings.TrimSpace(req.Env)

	rawEvents, err := decodeEventsPayload(req.Events)
	if err != nil {
		return nil, err
	}

	var accepted, rejected int
	for _, entry := range rawEvents {
		event, buildErr := buildBehaviorEvent(entry, gameID, env, time.Now().UTC())
		if buildErr != nil {
			rejected++
			continue
		}
		if err := l.svcCtx.BehaviorModel.RecordEvent(l.ctx, event); err != nil {
			rejected++
			continue
		}
		accepted++
	}

	return &types.IngestResponse{
		Accepted: accepted,
		Rejected: rejected,
		BatchId:  uuid.NewString(),
	}, nil
}
