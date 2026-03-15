package runtime

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"
)

func TestBuildRuntimeBindings_Default(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "official.notify",
	})
	if len(out) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(out))
	}
	if out[0].BindingKey != "official.notify.default" {
		t.Fatalf("unexpected default binding key: %s", out[0].BindingKey)
	}
}

func TestBuildRuntimeBindings_OfficialAnalytics(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "official.analytics",
	})
	if len(out) < 4 {
		t.Fatalf("expected analytics bindings, got %d", len(out))
	}
	keys := map[string]bool{}
	for _, b := range out {
		keys[b.BindingKey] = true
	}
	if !keys["analytics.filters.get"] || !keys["analytics.filters.update"] || !keys["analytics.ingest.batch"] {
		t.Fatalf("missing analytics binding keys: %+v", keys)
	}
}
