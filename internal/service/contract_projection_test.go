package service

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

// F13：契约 Diagnostics（含 schema_breaking_change）透传到 FunctionSpec，
// descriptors API 由此携带告警给 Dashboard。
func TestFunctionSpecFromContract_Diagnostics(t *testing.T) {
	diagnostics := []map[string]interface{}{
		{
			"code":     "schema_breaking_change",
			"severity": "warning",
			"message":  "inputSchema$/reason: 已声明的字段被删除",
			"field":    "inputSchema",
		},
	}
	raw, err := json.Marshal(diagnostics)
	if err != nil {
		t.Fatalf("marshal diagnostics: %v", err)
	}

	contract := &model.FunctionContract{
		GameID:      "game-1",
		Env:         "prod",
		FunctionID:  "player.ban",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "player",
		Capability:  dbenum.CapabilityAction,
		Execution:   "sync",
		Diagnostics: model.JSON(raw),
	}

	fnSpec := FunctionSpecFromContract(contract)
	if len(fnSpec.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(fnSpec.Diagnostics))
	}
	if fnSpec.Diagnostics[0].Code != "schema_breaking_change" {
		t.Fatalf("unexpected code: %s", fnSpec.Diagnostics[0].Code)
	}
	if fnSpec.Diagnostics[0].Severity != spec.SeverityWarning {
		t.Fatalf("expected warning severity, got %s", fnSpec.Diagnostics[0].Severity)
	}
	if fnSpec.Diagnostics[0].Message == "" {
		t.Fatal("expected message to be carried")
	}

	// 非法 JSON 不炸、返回 nil
	contract.Diagnostics = model.JSON("not-json")
	if diags := DiagnosticsFromJSON(json.RawMessage(contract.Diagnostics)); diags != nil {
		t.Fatalf("expected nil for invalid diagnostics JSON, got %+v", diags)
	}
	if diags := DiagnosticsFromJSON(nil); diags != nil {
		t.Fatalf("expected nil for empty diagnostics, got %+v", diags)
	}
}

// timeoutMs 漏投影回归：FunctionContract.TimeoutMs（0016 列）必须进入
// FunctionSpec，否则 descriptors API 的 timeoutMs 恒空、UI 不可见。
func TestFunctionSpecFromContract_TimeoutMs(t *testing.T) {
	contract := &model.FunctionContract{
		GameID:     "game-1",
		Env:        "prod",
		FunctionID: "player.ban",
		Version:    "1.0.0",
		TimeoutMs:  30000,
	}
	fnSpec := FunctionSpecFromContract(contract)
	if fnSpec.TimeoutMs != 30000 {
		t.Fatalf("expected TimeoutMs 30000, got %d", fnSpec.TimeoutMs)
	}

	contract.TimeoutMs = 0
	if got := FunctionSpecFromContract(contract).TimeoutMs; got != 0 {
		t.Fatalf("undeclared timeout must project 0, got %d", got)
	}
}
