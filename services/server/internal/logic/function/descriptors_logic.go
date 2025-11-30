// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DescriptorsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数描述符列表
func NewDescriptorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DescriptorsLogic {
	return &DescriptorsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DescriptorsLogic) Descriptors(req *types.DescriptorsRequest) (*types.DescriptorsResponse, error) {
	category := strings.TrimSpace(req.Type)
	descs, err := l.svcCtx.FunctionModel.ListDescriptorTemplates(l.ctx, category)
	if err != nil {
		return nil, err
	}

	items := make([]types.Descriptor, 0, len(descs))
	for _, desc := range descs {
		items = append(items, types.Descriptor{
			Id:          desc.DescriptorID,
			Name:        desc.Name,
			Description: desc.Description,
			Category:    desc.Category,
			Schema:      desc.Schema,
		})
	}

	return &types.DescriptorsResponse{
		Items: items,
	}, nil
}
