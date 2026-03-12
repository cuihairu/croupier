// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package terms

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TermDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除术语
func NewTermDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TermDeleteLogic {
	return &TermDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TermDeleteLogic) TermDelete(req *types.TermDeleteRequest) (resp *types.TermDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
