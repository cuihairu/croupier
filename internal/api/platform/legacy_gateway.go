package platform

import (
	"context"
	"strings"

	plat "github.com/cuihairu/croupier/internal/platform"
	"github.com/cuihairu/croupier/internal/svc"
)

type legacyPlatformProviderInfo struct {
	Name    string
	Enabled bool
	Methods []string
}

type legacyPlatformGateway interface {
	Call(ctx context.Context, platform, method string, request []byte) ([]byte, error)
	ListProviders() []legacyPlatformProviderInfo
	ListMethods(platform string) ([]string, bool)
	Reload(ctx context.Context) error
}

type loaderLegacyPlatformGateway struct {
	loader *plat.Loader
}

func newLegacyPlatformGateway(svcCtx *svc.ServiceContext) legacyPlatformGateway {
	if svcCtx == nil || svcCtx.PlatformLoader == nil {
		return nil
	}
	return &loaderLegacyPlatformGateway{loader: svcCtx.PlatformLoader}
}

func (g *loaderLegacyPlatformGateway) Call(ctx context.Context, platform, method string, request []byte) ([]byte, error) {
	if g == nil || g.loader == nil {
		return nil, nil
	}
	return g.loader.Registry().Call(ctx, platform, method, request)
}

func (g *loaderLegacyPlatformGateway) ListProviders() []legacyPlatformProviderInfo {
	if g == nil || g.loader == nil {
		return nil
	}
	providers := g.loader.ListProviders()
	out := make([]legacyPlatformProviderInfo, 0, len(providers))
	for _, p := range providers {
		if p == nil {
			continue
		}
		out = append(out, legacyPlatformProviderInfo{
			Name:    strings.TrimSpace(p.Name()),
			Enabled: p.IsEnabled(),
			Methods: p.SupportedMethods(),
		})
	}
	return out
}

func (g *loaderLegacyPlatformGateway) ListMethods(platform string) ([]string, bool) {
	if g == nil || g.loader == nil {
		return nil, false
	}
	provider, ok := g.loader.GetProvider(platform)
	if !ok || provider == nil {
		return nil, false
	}
	return provider.SupportedMethods(), true
}

func (g *loaderLegacyPlatformGateway) Reload(ctx context.Context) error {
	if g == nil || g.loader == nil {
		return nil
	}
	return g.loader.Reload(ctx)
}
