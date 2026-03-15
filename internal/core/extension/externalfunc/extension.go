package externalfunc

import "strings"

func IsExternalPlatformExtensionID(extensionID string) bool {
	key := strings.ToLower(strings.TrimSpace(extensionID))
	return strings.Contains(key, "external-platform") || strings.HasSuffix(key, ".external")
}
