package platform

import (
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestStringInSlice_Found(t *testing.T) {
	list := []string{"method1", "method2", "method3"}

	if !stringInSlice(list, "method1") {
		t.Fatal("expected to find method1 in list")
	}
	if !stringInSlice(list, "METHOD2") { // case insensitive
		t.Fatal("expected to find METHOD2 (case insensitive)")
	}
	if !stringInSlice(list, "  method1  ") { // with whitespace
		t.Fatal("expected to find method1 with whitespace")
	}
}

func TestStringInSlice_NotFound(t *testing.T) {
	list := []string{"method1", "method2"}

	if stringInSlice(list, "method3") {
		t.Fatal("expected not to find method3 in list")
	}
	if stringInSlice(list, "") {
		t.Fatal("expected not to find empty string")
	}
}

func TestStringInSlice_EmptyList(t *testing.T) {
	list := []string{}

	if stringInSlice(list, "method1") {
		t.Fatal("expected not to find in empty list")
	}
}

func TestStringInSlice_NilList(t *testing.T) {
	// Test with nil list (should not panic)
	var list []string
	result := stringInSlice(list, "test")
	if result {
		t.Fatal("expected false for nil list")
	}
}

func TestStringInSlice_EmptyStringInList(t *testing.T) {
	list := []string{"", "method1", "method2"}
	if !stringInSlice(list, "") {
		t.Fatal("expected to find empty string in list")
	}
}

func TestStringInSlice_WhitespaceStrings(t *testing.T) {
	list := []string{"  method1  ", "  method2  ", "method3"}
	if !stringInSlice(list, "method1") {
		t.Fatal("expected to find method1 with trimmed whitespace")
	}
}

// TestStringInSlice tests the stringInSlice helper function
func TestStringInSliceCheck(t *testing.T) {
	list := []string{"method1", "method2", "method3"}

	if !stringInSlice(list, "method1") {
		t.Fatal("expected to find method1")
	}
	if !stringInSlice(list, "METHOD1") {
		t.Fatal("expected to find METHOD1 (case insensitive)")
	}
	if stringInSlice(list, "method4") {
		t.Fatal("did not expect to find method4")
	}
	if !stringInSlice(list, "  method1  ") {
		t.Fatal("expected to find trimmed method1")
	}
}

// TestParseExternalFunctionID tests parsing external function IDs

// TestStringInSlice tests the stringInSlice helper function
func TestStringInSlice2(t *testing.T) {
	tests := []struct {
		name     string
		list     []string
		target   string
		expected bool
	}{
		{
			name:     "item exists",
			list:     []string{"method1", "method2", "method3"},
			target:   "method2",
			expected: true,
		},
		{
			name:     "item does not exist",
			list:     []string{"method1", "method2", "method3"},
			target:   "method4",
			expected: false,
		},
		{
			name:     "case insensitive match",
			list:     []string{"Method1", "Method2"},
			target:   "method1",
			expected: true,
		},
		{
			name:     "whitespace trimmed",
			list:     []string{"method1", " method2 "},
			target:   "method2",
			expected: true,
		},
		{
			name:     "empty list",
			list:     []string{},
			target:   "method1",
			expected: false,
		},
		{
			name:     "target with whitespace",
			list:     []string{"method1", "method2"},
			target:   " method1 ",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringInSlice(tt.list, tt.target)
			if result != tt.expected {
				t.Errorf("stringInSlice(%v, %q) = %v, want %v", tt.list, tt.target, result, tt.expected)
			}
		})
	}
}

// TestResolveMethodsSource2 tests the resolveMethodsSource helper function

// TestStringInSlice_VerifiesCaseInsensitiveMatching verifies case insensitive matching
func TestStringInSlice_VerifiesCaseInsensitiveMatching(t *testing.T) {
	list := []string{"Method1", "Method2", "Method3"}

	// All these should match (case insensitive)
	if !stringInSlice(list, "method1") {
		t.Error("expected to find 'method1' (case insensitive)")
	}
	if !stringInSlice(list, "METHOD1") {
		t.Error("expected to find 'METHOD1' (case insensitive)")
	}
	if !stringInSlice(list, "MeThOd1") {
		t.Error("expected to find 'MeThOd1' (case insensitive)")
	}
}

// TestStringInSlice_VerifiesWhitespaceTrimming verifies whitespace trimming

// TestStringInSlice_VerifiesWhitespaceTrimming verifies whitespace trimming
func TestStringInSlice_VerifiesWhitespaceTrimming(t *testing.T) {
	list := []string{"method1", "method2"}

	// These should match after trimming
	if !stringInSlice(list, " method1 ") {
		t.Error("expected to find ' method1 ' (with whitespace)")
	}
	if !stringInSlice(list, "  method2  ") {
		t.Error("expected to find '  method2  ' (with whitespace)")
	}
}

// TestListMethods_VerifiesMethodDeduplication verifies method deduplication
