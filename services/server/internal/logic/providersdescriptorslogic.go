// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersDescriptorsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProvidersDescriptorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersDescriptorsLogic {
	return &ProvidersDescriptorsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersDescriptorsLogic) ProvidersDescriptors() (*types.ProvidersDescriptorsResponse, error) {
	store := l.svcCtx.RegistryStore
	if store == nil {
		return nil, errRegistryUnavailable
	}
	caps := store.ListProviderCaps()
	out := make([]types.ProviderDescriptor, 0, len(caps))
	for _, cap := range caps {
		var manifest interface{}
		_ = json.Unmarshal(cap.Manifest, &manifest)
		out = append(out, types.ProviderDescriptor{
			Id:        cap.ID,
			Version:   cap.Version,
			Lang:      cap.Lang,
			Sdk:       cap.SDK,
			Manifest:  manifest,
			UpdatedAt: cap.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return &types.ProvidersDescriptorsResponse{Providers: out}, nil
}
