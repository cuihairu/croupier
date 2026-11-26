package platform

import (
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestBuildExternalFunctionID(t *testing.T) {
	got := buildExternalFunctionID("One Panel", "Install/App")
	if got != "external.one_panel.install_app" {
		t.Fatalf("unexpected external function id: %s", got)
	}
}

func TestBuildExternalFunctionID_SpecialCharacters(t *testing.T) {
	got := buildExternalFunctionID("One-Panel", "Install/App")
	// Special characters like hyphens are not removed
	if got != "external.one-panel.install_app" {
		t.Fatalf("unexpected external function id with special chars: %s", got)
	}
}

func TestBuildExternalFunctionID_MultipleWords(t *testing.T) {
	got := buildExternalFunctionID("My Cloud Provider", "Create/VM")
	// Hyphens and spaces are handled specifically
	if got != "external.my_cloud_provider.create_vm" {
		t.Fatalf("unexpected external function id: %s", got)
	}
}

func TestParseExternalFunctionID_Valid(t *testing.T) {
	provider, method, ok := parseExternalFunctionID("external.onepanel.install_app")

	if !ok {
		t.Fatal("expected ok=true for valid external function ID")
	}
	if provider != "onepanel" {
		t.Fatalf("expected provider=onepanel, got %s", provider)
	}
	if method != "install_app" {
		t.Fatalf("expected method=install_app, got %s", method)
	}
}

func TestParseExternalFunctionID_NotExternal(t *testing.T) {
	_, _, ok := parseExternalFunctionID("test.echo")

	if ok {
		t.Fatal("expected ok=false for non-external function ID")
	}
}

func TestParseExternalFunctionID_Empty(t *testing.T) {
	_, _, ok := parseExternalFunctionID("")

	if ok {
		t.Fatal("expected ok=false for empty function ID")
	}
}

// TestParseExternalFunctionID tests parsing external function IDs
func TestParseExternalFunctionIDCheck(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOk   bool
		wantProv string
		wantMeth string
	}{
		{
			name:     "valid function ID",
			input:    "external.myplatform.mymethod",
			wantOk:   true,
			wantProv: "myplatform",
			wantMeth: "mymethod",
		},
		{
			name:     "valid with underscores",
			input:    "external.my_platform.my_method",
			wantOk:   true,
			wantProv: "my_platform",
			wantMeth: "my_method",
		},
		{
			name:   "missing external prefix",
			input:  "myplatform.mymethod",
			wantOk: false,
		},
		{
			name:   "only external prefix",
			input:  "external",
			wantOk: false,
		},
		{
			name:   "empty string",
			input:  "",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, meth, ok := parseExternalFunctionID(tt.input)
			if ok != tt.wantOk {
				t.Fatalf("parseExternalFunctionID(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if tt.wantOk {
				if prov != tt.wantProv || meth != tt.wantMeth {
					t.Fatalf("parseExternalFunctionID(%q) = (%q, %q), want (%q, %q)", tt.input, prov, meth, tt.wantProv, tt.wantMeth)
				}
			}
		})
	}
}

// TestExtractPlatformMethodsFromBindingsDetailed tests extracting platform methods from bindings

// TestBuildExternalFunctionID_EmptyInputs tests buildExternalFunctionID with empty inputs
func TestBuildExternalFunctionID_EmptyInputs(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		method   string
		expected string
	}{
		{"empty platform", "", "method", ""},
		{"empty method", "platform", "", ""},
		{"both empty", "", "", ""},
		{"whitespace platform", "   ", "method", ""},
		{"whitespace method", "platform", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildExternalFunctionID(tt.platform, tt.method)
			if result != tt.expected {
				t.Errorf("buildExternalFunctionID(%q, %q) = %q, want %q", tt.platform, tt.method, result, tt.expected)
			}
		})
	}
}

// TestCall_WithRequestSet tests the request data conversion path
