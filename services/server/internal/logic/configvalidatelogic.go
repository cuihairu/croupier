package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigValidateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigValidateLogic {
	return &ConfigValidateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigValidateLogic) ConfigValidate(req *types.ConfigValidateRequest) (*types.ConfigValidateResponse, error) {
	if req == nil {
		return nil, ErrInvalidRequest
	}
	errs := validateConfigContent(req.Format, req.Content)
	return &types.ConfigValidateResponse{
		Valid:  len(errs) == 0,
		Errors: errs,
	}, nil
}
