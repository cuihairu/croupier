package meta

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// Root returns API information and version
func (s *Service) Root(ctx context.Context) (*RootResponse, error) {
	profiles := make([]string, 0, len(s.svcCtx.Config.Profiles))
	for name := range s.svcCtx.Config.Profiles {
		profiles = append(profiles, name)
	}
	sort.Strings(profiles)

	return &RootResponse{
		Service:     "croupier-server",
		Version:     currentAPIVersion(),
		Environment: s.svcCtx.Config.Server.Mode,
		Timestamp:   utils.FormatTimestamp(time.Now()),
		Features:    enabledFeatures(s.svcCtx.Config.FeatureFlags, settings.Current()),
		Profiles:    profiles,
		Links: map[string]string{
			"docs":   "https://github.com/cuihairu/croupier",
			"status": "/api/v1/ops/config",
			"health": "/api/v1/ops/health",
		},
	}, nil
}

// enabledFeatures reports the product domains enabled by the composed
// feature flags: L2 (config featureFlags, deployment trim) AND L3
// (platform_settings features.*, runtime soft switch). Always-on platform
// capabilities (functions/registry) are always listed.
func enabledFeatures(flags config.FeatureFlagsConfig, layered *settings.Layered) []string {
	features := []string{"alerts", "functions", "registry"}
	for _, domain := range []string{
		config.FlagDev,
		config.FlagSupport,
		config.FlagAnalytics,
		config.FlagOps,
		config.FlagExtensions,
	} {
		// L2 未裁剪的前提下读合成值（L3 可运行时关闭）。
		if flags.Enabled(domain) && layered.FeatureEnabled(domain) {
			features = append(features, domain)
		}
	}
	sort.Strings(features)
	return features
}

var (
	versionOnce sync.Once
	apiVersion  string
)

func currentAPIVersion() string {
	versionOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("CROUPIER_VERSION")); v != "" {
			apiVersion = v
			return
		}
		if v := readVersionFile(); v != "" {
			apiVersion = v
			return
		}
		apiVersion = "dev"
	})
	return apiVersion
}

func readVersionFile() string {
	data, err := os.ReadFile("VERSION")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
