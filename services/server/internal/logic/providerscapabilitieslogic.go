// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersCapabilitiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProvidersCapabilitiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersCapabilitiesLogic {
	return &ProvidersCapabilitiesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersCapabilitiesLogic) ProvidersCapabilities(req *types.ProvidersCapabilitiesRequest) (resp *types.ProvidersCapabilitiesResponse, err error) {
	if req == nil || strings.TrimSpace(req.Provider.Id) == "" {
		return nil, errors.New("provider id required")
	}
	if len(req.Manifest) == 0 {
		return nil, errors.New("manifest required")
	}
	if len(req.Manifest) > 10*1024*1024 {
		return nil, fmt.Errorf("manifest too large")
	}
	if err := validateManifestJSON(req.Manifest); err != nil {
		return nil, fmt.Errorf("manifest invalid: %w", err)
	}
	store := l.svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry unavailable")
	}
	store.UpsertProviderCaps(registry.ProviderCaps{
		ID:       req.Provider.Id,
		Version:  req.Provider.Version,
		Lang:     req.Provider.Lang,
		SDK:      req.Provider.Sdk,
		Manifest: append([]byte(nil), req.Manifest...),
	})
	l.svcCtx.MergeProviderFunctions(req.Manifest)
	return &types.ProvidersCapabilitiesResponse{Ok: true}, nil
}
