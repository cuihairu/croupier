package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsBackupDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsBackupDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupDeleteLogic {
	return &OpsBackupDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupDeleteLogic) OpsBackupDelete(id string) (*types.GenericOkResponse, error) {
	if id == "" {
		return nil, ErrInvalidRequest
	}
	if !l.svcCtx.DeleteBackup(id) {
		return nil, errors.New("backup not found")
	}
	return &types.GenericOkResponse{Ok: true}, nil
}
