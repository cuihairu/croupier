package platform

import (
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestResolveMethodsSource(t *testing.T) {
	if got := resolveMethodsSource(true); got != "extension" {
		t.Fatalf("expected extension, got=%s", got)
	}
	if got := resolveMethodsSource(false); got != "" {
		t.Fatalf("expected empty source, got=%s", got)
	}
}

// TestResolveMethodsSource2 tests the resolveMethodsSource helper function
func TestResolveMethodsSource2(t *testing.T) {
	t.Run("with extension", func(t *testing.T) {
		result := resolveMethodsSource(true)
		if result != "extension" {
			t.Errorf("resolveMethodsSource(true) = %q, want \"extension\"", result)
		}
	})

	t.Run("without extension", func(t *testing.T) {
		result := resolveMethodsSource(false)
		if result != "" {
			t.Errorf("resolveMethodsSource(false) = %q, want \"\"", result)
		}
	})
}

// TestCall_WithEmptyResponseFromDispatcher tests when dispatcher returns empty response
