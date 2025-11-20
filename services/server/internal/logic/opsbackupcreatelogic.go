package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsBackupCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsBackupCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupCreateLogic {
	return &OpsBackupCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupCreateLogic) OpsBackupCreate(req *types.OpsBackupCreateRequest) (*types.OpsBackupCreateResponse, error) {
	if req == nil || strings.TrimSpace(req.Kind) == "" {
		return nil, ErrInvalidRequest
	}
	entry, err := l.svcCtx.CreateBackup(req.Kind, req.Target)
	if err != nil {
		return nil, err
	}
	return &types.OpsBackupCreateResponse{Id: entry.ID}, nil
}
