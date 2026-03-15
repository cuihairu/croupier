package externalfunc

import "testing"

func TestBuildFunctionID(t *testing.T) {
	got := BuildFunctionID("One Panel", "Install/App")
	if got != "external.one_panel.install_app" {
		t.Fatalf("unexpected function id: %s", got)
	}
}

func TestParseFunctionID(t *testing.T) {
	p, m, ok := ParseFunctionID("external.onepanel.install_app")
	if !ok {
		t.Fatalf("expected parse success")
	}
	if p != "onepanel" || m != "install_app" {
		t.Fatalf("unexpected parse result provider=%s method=%s", p, m)
	}
}

func TestSanitizeKey(t *testing.T) {
	got := SanitizeKey("  A/B C  ")
	if got != "a_b_c" {
		t.Fatalf("unexpected sanitize result: %s", got)
	}
}
