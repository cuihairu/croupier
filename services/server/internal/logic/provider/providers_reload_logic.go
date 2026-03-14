// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type ProvidersReloadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重新加载提供者
func NewProvidersReloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersReloadLogic {
	return &ProvidersReloadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersReloadLogic) ProvidersReload(req *types.ProviderActionRequest) (resp *types.ProviderReloadResponse, err error) {
	caps, err := getProviderCaps(l.svcCtx.RegistryStore, req.ID)
	if err != nil {
		return nil, err
	}

	if _, err := decodeOpenAPIDoc(caps.OpenAPIDoc); err != nil {
		return nil, err
	}

	refreshProviderTimestamp(l.svcCtx.RegistryStore, caps)

	return &types.ProviderReloadResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id":        caps.ID,
			"updatedAt": time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}
