package openapi

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/drivers"
)

func TestDriverFactoryRegistersOpenAPI(t *testing.T) {
	if _, err := drivers.New("openapi", drivers.Config{
		Endpoint: "http://127.0.0.1:1",
		Extra: map[string]string{
			"methods": `[{"name":"day_report","path":"/day_report","method":"POST"}]`,
		},
	}); err != nil {
		t.Fatalf("New(openapi): %v", err)
	}
}

func TestDriverFactoryInitializesOpenAPI(t *testing.T) {
	driver, err := drivers.NewWithContext(context.Background(), "openapi", drivers.Config{
		Endpoint: "http://127.0.0.1:1",
		Extra: map[string]string{
			"methods": `[{"name":"day_report","path":"/day_report","method":"POST"}]`,
		},
	})
	if err != nil {
		t.Fatalf("NewWithContext: %v", err)
	}
	provider, ok := driver.(interface {
		Kind() string
		SupportedMethods() []string
	})
	if !ok {
		t.Fatalf("driver does not expose openapi methods")
	}
	if provider.Kind() != "openapi" {
		t.Fatalf("Kind = %q", provider.Kind())
	}
	methods := provider.SupportedMethods()
	if len(methods) != 1 || methods[0] != "day_report" {
		t.Fatalf("SupportedMethods = %#v", methods)
	}
	if err := driver.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
