// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package assignment

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssignmentsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新分配
func NewAssignmentsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignmentsUpdateLogic {
	return &AssignmentsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AssignmentsUpdateLogic) AssignmentsUpdate(req *types.AssignmentsUpdateRequest) (resp *types.AssignmentsUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
