package logic

import (
	"context"

	"github.com/cuihairu/croupier/internal/function/descriptor"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DescriptorsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDescriptorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DescriptorsLogic {
	return &DescriptorsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DescriptorsLogic) Descriptors(req *types.DescriptorsQuery) (*types.DescriptorsResponse, error) {
	var snapshot []*descriptor.Descriptor
	if l.svcCtx != nil {
		snapshot = l.svcCtx.DescriptorsSnapshot()
	}
	if req != nil && req.Detailed {
		var manifests interface{}
		if l.svcCtx != nil && l.svcCtx.RegistryStore != nil {
			manifests = l.svcCtx.RegistryStore.BuildUnifiedDescriptors()
		}
		return &types.DescriptorsResponse{
			LegacyDescriptors: snapshot,
			ProviderManifests: manifests,
		}, nil
	}
	return &types.DescriptorsResponse{Descriptors: snapshot}, nil
}
