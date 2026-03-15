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
	if !keys["analytics.overview"] || !keys["analytics.realtime"] || !keys["analytics.retention"] || !keys["analytics.payments"] {
		t.Fatalf("missing analytics page bindings: %+v", keys)
	}
}

func TestBuildRuntimeBindings_OfficialAlerting(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "official.alerting",
	})
	if len(out) < 4 {
		t.Fatalf("expected alerting bindings, got %d", len(out))
	}
	keys := map[string]bool{}
	for _, b := range out {
		keys[b.BindingKey] = true
	}
	if !keys["alerts.overview"] || !keys["alerts.management"] || !keys["alerts.list"] || !keys["alerts.silence"] {
		t.Fatalf("missing alerting binding keys: %+v", keys)
	}
}

func TestBuildRuntimeBindings_OfficialNotification(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "official.notification",
	})
	if len(out) < 4 {
		t.Fatalf("expected notification bindings, got %d", len(out))
	}
	keys := map[string]bool{}
	for _, b := range out {
		keys[b.BindingKey] = true
	}
	if !keys["notifications.overview"] || !keys["notifications.management"] || !keys["notifications.get"] || !keys["notifications.update"] {
		t.Fatalf("missing notification binding keys: %+v", keys)
	}
}

func TestBuildRuntimeBindings_OfficialApproval(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "official.approval",
	})
	if len(out) < 4 {
		t.Fatalf("expected approval bindings, got %d", len(out))
	}
	keys := map[string]bool{}
	for _, b := range out {
		keys[b.BindingKey] = true
	}
	if !keys["approvals.overview"] || !keys["approvals.management"] || !keys["approvals.approve"] || !keys["approvals.reject"] {
		t.Fatalf("missing approval binding keys: %+v", keys)
	}
}

func TestBuildRuntimeBindings_OfficialBackupAdvanced(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "official.backup-advanced",
	})
	if len(out) < 4 {
		t.Fatalf("expected backup bindings, got %d", len(out))
	}
	keys := map[string]bool{}
	for _, b := range out {
		keys[b.BindingKey] = true
	}
	if !keys["backups.overview"] || !keys["backups.management"] || !keys["backups.create"] || !keys["backups.delete"] {
		t.Fatalf("missing backup binding keys: %+v", keys)
	}
}
