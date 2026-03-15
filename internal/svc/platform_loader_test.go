package svc

import (
	"testing"

	"github.com/cuihairu/croupier/internal/config"
)

func TestInitPlatformLoaderSkippedInExtensionOnly(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "true")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_DISABLED", "")
	cfg := config.Config{
		Platforms: config.PlatformConfig{
			Enabled:    true,
			ConfigFile: "configs/platforms.yaml",
		},
	}
	loader := initPlatformLoader(cfg)
	if loader != nil {
		t.Fatalf("expected loader=nil in extension-only mode")
	}
}
