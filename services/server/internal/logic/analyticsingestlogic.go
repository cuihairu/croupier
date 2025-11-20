package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsIngestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsIngestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsIngestLogic {
	return &AnalyticsIngestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsIngestLogic) AnalyticsIngest(req *types.AnalyticsIngestRequest) error {
	queue := l.svcCtx.AnalyticsQueue()
	if queue == nil {
		return nil
	}
	for _, evt := range req.Events {
		if evt == nil {
			continue
		}
		if err := queue.PublishEvent(evt); err != nil {
			return err
		}
	}
	return nil
}

type AnalyticsPaymentsIngestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsPaymentsIngestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsPaymentsIngestLogic {
	return &AnalyticsPaymentsIngestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsPaymentsIngestLogic) AnalyticsPaymentsIngest(req *types.AnalyticsPaymentsIngestRequest) error {
	queue := l.svcCtx.AnalyticsQueue()
	if queue == nil {
		return nil
	}
	for _, evt := range req.Events {
		if evt == nil {
			continue
		}
		if err := queue.PublishPayment(evt); err != nil {
			return err
		}
	}
	return nil
}
