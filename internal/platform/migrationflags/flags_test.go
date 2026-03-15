package migrationflags

import "testing"

func TestUseLegacyFallback(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_DISABLED", "")
	if !UseLegacyFallback() {
		t.Fatalf("expected legacy fallback enabled by default")
	}
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "true")
	if UseLegacyFallback() {
		t.Fatalf("expected legacy fallback disabled in extension-only")
	}
	t.Setenv("CROUPIER_PLATFORM_EXTENSION_ONLY", "")
	t.Setenv("CROUPIER_PLATFORM_LEGACY_DISABLED", "true")
	if UseLegacyFallback() {
		t.Fatalf("expected legacy fallback disabled when legacy-disabled=true")
	}
}

func TestAllowLegacyFallbackAfterExtensionError(t *testing.T) {
	t.Setenv("CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR", "")
	if !AllowLegacyFallbackAfterExtensionError() {
		t.Fatalf("expected default true")
	}
	t.Setenv("CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR", "false")
	if AllowLegacyFallbackAfterExtensionError() {
		t.Fatalf("expected false")
	}
}
