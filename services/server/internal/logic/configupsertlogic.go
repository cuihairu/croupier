package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigUpsertLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigUpsertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigUpsertLogic {
	return &ConfigUpsertLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigUpsertLogic) ConfigUpsert(req *types.ConfigUpsertRequest) (*types.ConfigUpsertResponse, error) {
	if req == nil {
		return nil, ErrInvalidRequest
	}
	input := svc.ConfigUpsertInput{
		GameID:      req.GameId,
		Env:         req.Env,
		Format:      req.Format,
		Content:     req.Content,
		Message:     req.Message,
		BaseVersion: req.BaseVersion,
		Editor:      svc.ActorFromContext(l.ctx),
	}
	record, err := l.svcCtx.UpsertConfig(req.Id, input)
	if err != nil {
		if errors.Is(err, svc.ErrConfigInvalidInput) {
			return nil, ErrInvalidRequest
		}
		return nil, err
	}
	return &types.ConfigUpsertResponse{
		Ok:      true,
		Version: record.Version,
		Etag:    record.ETag,
	}, nil
}
