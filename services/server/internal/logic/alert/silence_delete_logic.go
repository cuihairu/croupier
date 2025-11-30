// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package alert

import (
	"context"
	"errors"
	"strconv"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SilenceDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除静默规则
func NewSilenceDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SilenceDeleteLogic {
	return &SilenceDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SilenceDeleteLogic) SilenceDelete(req *types.SilenceDeleteRequest) error {
	if l.svcCtx.AlertModel == nil {
		return errors.New("告警模型未初始化")
	}
	if req == nil {
		return errors.New("请求体不能为空")
	}

	id, err := strconv.ParseUint(req.ID, 10, 64)
	if err != nil {
		return errors.New("静默ID格式不正确")
	}

	return l.svcCtx.AlertModel.DeleteSilence(l.ctx, uint(id))
}
