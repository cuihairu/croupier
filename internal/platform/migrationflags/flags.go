package migrationflags

import (
	"os"
	"strings"
)

func IsExtensionOnly() bool {
	return envBool("CROUPIER_PLATFORM_EXTENSION_ONLY")
}

func IsLegacyDisabled() bool {
	return envBool("CROUPIER_PLATFORM_LEGACY_DISABLED")
}

func UseLegacyFallback() bool {
	return !IsExtensionOnly() && !IsLegacyDisabled()
}

func AllowLegacyFallbackAfterExtensionError() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR")))
	if v == "" {
		return true
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
