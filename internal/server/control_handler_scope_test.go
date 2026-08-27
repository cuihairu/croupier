package server

import (
	"context"
	"testing"

	gosqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

func newAlertTestModel(t *testing.T) *model.AlertModel {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open(t.TempDir()+"/alert.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Alert{}); err != nil {
		t.Fatal(err)
	}
	return model.NewAlertModel(db)
}

func newScopeTestService(t *testing.T) (*ControlService, *model.AlertModel) {
	t.Helper()
	svc := NewControlService(registry.NewStore(), nil)
	am := newAlertTestModel(t)
	svc.SetAlertModel(am)
	return svc, am
}

// SDK/自定义游戏服注册时 scope 与 agent 不一致 → 平台告警 firing。
func TestValidateProviderScope_MismatchFiresAlert(t *testing.T) {
	svc, am := newScopeTestService(t)
	req := &agentv1.RegisterRequest{AgentId: "agent-1", GameId: "demo-game", Env: "development"}
	p := &agentv1.AgentProcess{ServiceId: "svc-1", GameId: "other-game", Env: "development"}
	var warnings []string

	svc.validateProviderScope(context.Background(), req, p, &warnings)

	if len(warnings) != 1 {
		t.Fatalf("warnings = %v", warnings)
	}
	got, err := am.FindByAlertID(context.Background(), "provider_scope_mismatch:agent-1:svc-1")
	if err != nil || got == nil {
		t.Fatalf("alert not created: %v", err)
	}
	if got.Status != "firing" || got.Level != "warning" {
		t.Fatalf("alert = %+v", got)
	}

	// 重复注册不重复建告警（alertID 唯一去重）。
	var w2 []string
	svc.validateProviderScope(context.Background(), req, p, &w2)
	list, total, _ := am.List(context.Background(), model.ListAlertsOptions{})
	if total != 1 || len(list) != 1 {
		t.Fatalf("duplicate alerts: total=%d len=%d", total, len(list))
	}
}

// provider 修正配置后再次注册一致 → 告警自动 resolved。
func TestValidateProviderScope_MatchResolvesAlert(t *testing.T) {
	svc, am := newScopeTestService(t)
	req := &agentv1.RegisterRequest{AgentId: "agent-1", GameId: "demo-game", Env: "development"}
	bad := &agentv1.AgentProcess{ServiceId: "svc-1", GameId: "other-game", Env: "development"}
	good := &agentv1.AgentProcess{ServiceId: "svc-1", GameId: "demo-game", Env: "development"}
	var warnings []string

	svc.validateProviderScope(context.Background(), req, bad, &warnings)
	if len(warnings) != 1 {
		t.Fatalf("expected mismatch warning, got %v", warnings)
	}

	var w2 []string
	svc.validateProviderScope(context.Background(), req, good, &w2)
	if len(w2) != 0 {
		t.Fatalf("no warnings expected on match, got %v", w2)
	}
	got, err := am.FindByAlertID(context.Background(), "provider_scope_mismatch:agent-1:svc-1")
	if err != nil || got == nil {
		t.Fatalf("alert missing: %v", err)
	}
	if got.Status != "resolved" {
		t.Fatalf("alert not resolved: %+v", got)
	}
}

// 向后兼容：provider 未传 scope（空值）不校验、不告警。
func TestValidateProviderScope_EmptyProviderScopeSkipped(t *testing.T) {
	svc, am := newScopeTestService(t)
	req := &agentv1.RegisterRequest{AgentId: "agent-1", GameId: "demo-game", Env: "development"}
	p := &agentv1.AgentProcess{ServiceId: "svc-1"}
	var warnings []string

	svc.validateProviderScope(context.Background(), req, p, &warnings)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if got, err := am.FindByAlertID(context.Background(), "provider_scope_mismatch:agent-1:svc-1"); err == nil && got != nil {
		t.Fatalf("unexpected alert: %+v", got)
	}
}

// 告警通道未注入（SetAlertModel 未调用）：警告仍进 warningTexts。
func TestValidateProviderScope_NoAlertModelStillWarns(t *testing.T) {
	svc := NewControlService(registry.NewStore(), nil)
	req := &agentv1.RegisterRequest{AgentId: "agent-1", GameId: "g", Env: "e"}
	p := &agentv1.AgentProcess{ServiceId: "svc-1", GameId: "other"}
	var warnings []string

	svc.validateProviderScope(context.Background(), req, p, &warnings)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v", warnings)
	}
}
