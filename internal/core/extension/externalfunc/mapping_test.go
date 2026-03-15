package externalfunc

import "testing"

func TestCapability(t *testing.T) {
	if got := Capability("One Panel"); got != "external.one_panel" {
		t.Fatalf("unexpected capability: %s", got)
	}
}

func TestOperation(t *testing.T) {
	if got := Operation("Install/App"); got != "install_app" {
		t.Fatalf("unexpected operation: %s", got)
	}
}

func TestCapabilityOperationFromFunctionID(t *testing.T) {
	capability, operation, ok := CapabilityOperationFromFunctionID("external.onepanel.install_app")
	if !ok {
		t.Fatalf("expected parse success")
	}
	if capability != "external.onepanel" || operation != "install_app" {
		t.Fatalf("unexpected mapping capability=%s operation=%s", capability, operation)
	}
}
