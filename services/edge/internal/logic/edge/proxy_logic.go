// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

import (
	"context"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProxyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProxyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProxyLogic {
	return &ProxyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProxyLogic) Proxy(req *types.ProxyRequest) (resp *types.ProxyResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
