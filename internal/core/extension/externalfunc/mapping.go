package externalfunc

import "strings"

func Capability(provider string) string {
	p := SanitizeKey(provider)
	if p == "" {
		return ""
	}
	return "external." + p
}

func Operation(method string) string {
	return SanitizeKey(method)
}

func CapabilityOperationFromFunctionID(functionID string) (capability string, operation string, ok bool) {
	provider, method, ok := ParseFunctionID(functionID)
	if !ok {
		return "", "", false
	}
	capability = Capability(provider)
	operation = Operation(method)
	if strings.TrimSpace(capability) == "" || strings.TrimSpace(operation) == "" {
		return "", "", false
	}
	return capability, operation, true
}
