package declare

import (
	"testing"

	"github.com/cuihairu/croupier/internal/function/registry"
)

// TestMetadataRegistersIntoSharedRegistry proves the reuse claim of
// todo.md:35: plugin-declared functions, once built into shared
// metadata, register into the SAME function registry the SDK/provider
// chain uses — presentation guard, security and behavior validation all
// apply unchanged (test-side import of internal is allowed; production
// wiring merges these through the sidecar function chain).
func TestMetadataRegistersIntoSharedRegistry(t *testing.T) {
	reg := registry.New()
	md, err := Metadata("httpget", "1.0.0", validDescriptor())
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	if err := reg.Register(t.Context(), md); err != nil {
		t.Fatalf("plugin metadata must register into the shared registry: %v", err)
	}
	got, err := reg.Get(t.Context(), "plugin.httpget.health")
	if err != nil || got == nil {
		t.Fatalf("registered function retrievable: %v %v", got, err)
	}
	if got.Resource != "plugin.httpget" {
		t.Fatalf("resource: %q", got.Resource)
	}
}
